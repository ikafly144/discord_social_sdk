package main

/*
#cgo LDFLAGS: -lclang
#include <clang-c/Index.h>
#include <stdlib.h>

enum CXChildVisitResult visitCursor(CXCursor cursor, CXCursor parent, CXClientData client_data);
enum CXChildVisitResult visitCursorEnum(CXCursor cursor, CXCursor parent, CXClientData client_data);
enum CXChildVisitResult visitCursorStruct(CXCursor cursor, CXCursor parent, CXClientData client_data);
*/
import "C"

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"os"
	"sort"
	"strings"
	"unsafe"
)

type Generator struct {
	enums     map[string]Enum
	structs   map[string]Struct
	functions []Function
	callbacks map[string]Callback
}

type Enum struct {
	Name   string
	Values []EnumValue
}

type EnumValue struct {
	Name  string
	Value int64
}

type Struct struct {
	Name     string
	IsOpaque bool
	Fields   []Field
}

type Field struct {
	Name string
	Type string
}

type Function struct {
	Name         string
	ReturnType   string
	CReturnType  string
	ReturnIsBool bool
	Args         []Arg
	OutArg       *Arg
}

type Arg struct {
	Name   string
	Type   string
	CType  string
	IsBool bool
}

type Callback struct {
	Name        string
	ReturnType  string
	CReturnType string
	Args        []Arg
	Signature   string // Unique signature for grouping gateways
}

var currentGenerator *Generator
var currentEnum *Enum
var currentStruct *Struct

//export goVisitCursor
func goVisitCursor(cursor, parent *C.CXCursor) C.enum_CXChildVisitResult {
	if C.clang_Location_isFromMainFile(C.clang_getCursorLocation(*cursor)) == 0 {
		return C.CXChildVisit_Continue
	}

	kind := C.clang_getCursorKind(*cursor)
	switch kind {
	case C.CXCursor_EnumDecl:
		currentGenerator.parseEnum(*cursor)
	case C.CXCursor_StructDecl:
		currentGenerator.parseStruct(*cursor)
	case C.CXCursor_FunctionDecl:
		currentGenerator.parseFunction(*cursor)
	case C.CXCursor_TypedefDecl:
		underlying := C.clang_getTypedefDeclUnderlyingType(*cursor)
		if underlying.kind == C.CXType_Pointer {
			pointee := C.clang_getPointeeType(underlying)
			if pointee.kind == C.CXType_FunctionProto {
				currentGenerator.parseCallback(*cursor)
			}
		}
	}

	return C.CXChildVisit_Continue
}

func getString(cxstr C.CXString) string {
	cstr := C.clang_getCString(cxstr)
	if cstr == nil {
		return ""
	}
	res := C.GoString(cstr)
	C.clang_disposeString(cxstr)
	return res
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("Usage: gen <header_file>")
	}
	headerFile := os.Args[1]

	idx := C.clang_createIndex(0, 0)
	defer C.clang_disposeIndex(idx)

	cHeaderFile := C.CString(headerFile)
	defer C.free(unsafe.Pointer(cHeaderFile))

	args := []*C.char{
		C.CString("-Iinclude"),
		C.CString("-std=c99"),
		C.CString("-xc"),
		C.CString("-Dbool=_Bool"),
	}
	defer func() {
		for _, arg := range args {
			C.free(unsafe.Pointer(arg))
		}
	}()

	tu := C.clang_parseTranslationUnit(idx, cHeaderFile, (**C.char)(unsafe.Pointer(&args[0])), C.int(len(args)), nil, 0, C.CXTranslationUnit_None)
	if tu == nil {
		log.Fatal("Failed to parse translation unit")
	}
	defer C.clang_disposeTranslationUnit(tu)

	currentGenerator = &Generator{
		enums:     make(map[string]Enum),
		structs:   make(map[string]Struct),
		callbacks: make(map[string]Callback),
	}

	cursor := C.clang_getTranslationUnitCursor(tu)
	C.clang_visitChildren(cursor, C.CXCursorVisitor(C.visitCursor), nil)

	currentGenerator.generate()
}

func (g *Generator) parseEnum(cursor C.CXCursor) {
	name := getString(C.clang_getCursorSpelling(cursor))
	if name == "" {
		return
	}
	if _, ok := g.enums[name]; ok {
		return
	}
	enum := Enum{Name: name}
	currentEnum = &enum

	C.clang_visitChildren(cursor, C.CXCursorVisitor(C.visitCursorEnum), nil)

	g.enums[name] = enum
	currentEnum = nil
}

//export goVisitCursorEnum
func goVisitCursorEnum(c, p *C.CXCursor, clientData unsafe.Pointer) C.enum_CXChildVisitResult {
	if C.clang_getCursorKind(*c) == C.CXCursor_EnumConstantDecl {
		currentEnum.Values = append(currentEnum.Values, EnumValue{
			Name:  getString(C.clang_getCursorSpelling(*c)),
			Value: int64(C.clang_getEnumConstantDeclValue(*c)),
		})
	}
	return C.CXChildVisit_Continue
}

func (g *Generator) parseStruct(cursor C.CXCursor) {
	name := getString(C.clang_getCursorSpelling(cursor))
	if name == "" {
		return
	}

	s := Struct{Name: name}
	currentStruct = &s
	C.clang_visitChildren(cursor, C.CXCursorVisitor(C.visitCursorStruct), nil)

	if len(s.Fields) == 1 && s.Fields[0].Name == "opaque" {
		s.IsOpaque = true
	}

	if _, ok := g.structs[name]; ok {
		if len(s.Fields) > 0 {
			g.structs[name] = s
		}
	} else {
		g.structs[name] = s
	}
	currentStruct = nil
}

//export goVisitCursorStruct
func goVisitCursorStruct(c, p *C.CXCursor, clientData unsafe.Pointer) C.enum_CXChildVisitResult {
	if C.clang_getCursorKind(*c) == C.CXCursor_FieldDecl {
		currentStruct.Fields = append(currentStruct.Fields, Field{
			Name: getString(C.clang_getCursorSpelling(*c)),
			Type: getString(C.clang_getTypeSpelling(C.clang_getCursorType(*c))),
		})
	}
	return C.CXChildVisit_Continue
}

func (g *Generator) parseFunction(cursor C.CXCursor) {
	name := getString(C.clang_getCursorSpelling(cursor))
	retType, cRetType, retIsBool := mapCTypeToGo(C.clang_getCursorResultType(cursor))
	f := Function{
		Name:         name,
		ReturnType:   retType,
		CReturnType:  cRetType,
		ReturnIsBool: retIsBool,
	}
	numArgs := int(C.clang_Cursor_getNumArguments(cursor))
	for i := 0; i < numArgs; i++ {
		argCursor := C.clang_Cursor_getArgument(cursor, C.uint(i))
		argName := getString(C.clang_getCursorSpelling(argCursor))
		argType, cArgType, argIsBool := mapCTypeToGo(C.clang_getCursorType(argCursor))
		
		arg := Arg{
			Name:   argName,
			Type:   argType,
			CType:  cArgType,
			IsBool: argIsBool,
		}

		if argName == "returnValue" {
			if strings.HasPrefix(arg.Type, "*") {
				arg.Type = strings.TrimPrefix(arg.Type, "*")
				arg.CType = strings.TrimSuffix(arg.CType, "*")
				arg.CType = strings.TrimSuffix(arg.CType, " ")
			}
			f.OutArg = &arg
		} else {
			f.Args = append(f.Args, arg)
		}
	}
	g.functions = append(g.functions, f)
}

func (g *Generator) parseCallback(cursor C.CXCursor) {
	name := getString(C.clang_getCursorSpelling(cursor))
	underlying := C.clang_getTypedefDeclUnderlyingType(cursor)
	proto := C.clang_getPointeeType(underlying)

	retGo, retC, _ := mapCTypeToGo(C.clang_getResultType(proto))
	cb := Callback{
		Name:        name,
		ReturnType:  retGo,
		CReturnType: retC,
	}

	numArgs := int(C.clang_getNumArgTypes(proto))
	var sigParts []string
	for i := 0; i < numArgs; i++ {
		argType := C.clang_getArgType(proto, C.uint(i))
		goType, cType, isBool := mapCTypeToGo(argType)
		argName := fmt.Sprintf("arg%d", i)
		if i == numArgs-1 && (cType == "void*" || cType == "void *") { argName = "userData" }
		cb.Args = append(cb.Args, Arg{
			Name:   argName,
			Type:   goType,
			CType:  cType,
			IsBool: isBool,
		})
		sigParts = append(sigParts, cType)
	}
	cb.Signature = strings.Join(sigParts, "_")
	cb.Signature = strings.ReplaceAll(cb.Signature, "*", "Ptr")
	cb.Signature = strings.ReplaceAll(cb.Signature, " ", "_")
	g.callbacks[name] = cb
}

func (g *Generator) isSpanType(name string) (bool, string) {
	if !strings.HasSuffix(name, "Span") { return false, "" }
	s, ok := g.structs[name]
	if !ok || len(s.Fields) != 2 { return false, "" }
	var hasPtr, hasSize bool
	var elemType string
	for _, f := range s.Fields {
		if f.Name == "ptr" { hasPtr = true; elemType = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(f.Type, "struct ")), "*"); elemType = strings.TrimSpace(elemType) }
		if f.Name == "size" { hasSize = true }
	}
	if hasPtr && hasSize {
		goElem := elemType
		if t, ok := mapSimpleCTypeToGo(elemType); ok { goElem = t }
		return true, goElem
	}
	return false, ""
}

func mapSimpleCTypeToGo(cType string) (string, bool) {
	switch cType {
	case "uint64_t": return "uint64", true
	case "int32_t": return "int32", true
	case "uint32_t": return "uint32", true
	case "int16_t": return "int16", true
	case "uint8_t": return "uint8", true
	case "float": return "float32", true
	case "double": return "float64", true
	case "bool", "_Bool": return "bool", true
	}
	return "", false
}

func mapCTypeToGo(t C.CXType) (goType, cType string, isBool bool) {
	kind := t.kind
	spelling := getString(C.clang_getTypeSpelling(t))
	stripConst := func(s string) string {
		s = strings.TrimPrefix(s, "const ")
		s = strings.TrimSuffix(s, " const")
		return s
	}
	cType = stripConst(spelling)
	if currentGenerator != nil {
		if ok, elem := currentGenerator.isSpanType(cType); ok { return "[]" + elem, cType, false }
	}
	canonical := C.clang_getCanonicalType(t)
	if cType == "bool" || cType == "_Bool" || canonical.kind == C.CXType_Bool {
		return "bool", "bool", true
	}
	goType = cType
	switch kind {
	case C.CXType_Bool:
		return "bool", "bool", true
	case C.CXType_Int, C.CXType_SChar, C.CXType_Short, C.CXType_Long, C.CXType_LongLong:
		if cType == "int8_t" { return "int8", "int8_t", false }
		if cType == "int16_t" { return "int16", "int16_t", false }
		if cType == "int32_t" { return "int32", "int32_t", false }
		if cType == "int64_t" { return "int64", "int64_t", false }
		return "int", cType, false
	case C.CXType_UInt, C.CXType_UChar, C.CXType_UShort, C.CXType_ULong, C.CXType_ULongLong:
		if cType == "uint8_t" { return "uint8", "uint8_t", false }
		if cType == "uint16_t" { return "uint16", "uint16_t", false }
		if cType == "uint32_t" { return "uint32", "uint32_t", false }
		if cType == "uint64_t" { return "uint64", "uint64_t", false }
		return "uint", cType, false
	case C.CXType_Float:
		return "float32", "float", false
	case C.CXType_Double:
		return "float64", "double", false
	case C.CXType_Pointer:
		pointee := C.clang_getPointeeType(t)
		if pointee.kind == C.CXType_Void { return "unsafe.Pointer", "void*", false }
		goPointee, cPointee, _ := mapCTypeToGo(pointee)
		if goPointee == "string" {
			return "string", cPointee + "*", false
		}
		return "*" + goPointee, cPointee + "*", false
	case C.CXType_Void:
		return "", "void", false
	case C.CXType_Typedef:
		if cType == "Discord_String" { return "string", "Discord_String", false }
		if cType == "size_t" { return "uintptr", "size_t", false }
		if cType == "uint64_t" { return "uint64", "uint64_t", false }
		if cType == "int32_t" { return "int32", "int32_t", false }
		if cType == "uint32_t" { return "uint32", "uint32_t", false }
		if cType == "uint8_t" { return "uint8", "uint8_t", false }
		if cType == "int16_t" { return "int16", "int16_t", false }
		return cType, cType, false
	case C.CXType_Record, C.CXType_Enum:
		return cType, cType, false
	}
	return goType, cType, false
}

var goKeywords = map[string]bool{
	"break": true, "default": true, "func": true, "interface": true, "select": true,
	"case": true, "defer": true, "go": true, "map": true, "struct": true,
	"chan": true, "else": true, "goto": true, "package": true, "switch": true,
	"const": true, "fallthrough": true, "if": true, "range": true, "type": true,
	"continue": true, "for": true, "import": true, "return": true, "var": true,
}

func sanitizeGoName(name string) string {
	if goKeywords[name] { return name + "_" }
	return name
}

func (g *Generator) generate() {
	var buf bytes.Buffer
	var cBuf bytes.Buffer
	buf.WriteString("// Code generated by gen; DO NOT EDIT.\n\npackage discord\n\n/*\n")
	buf.WriteString("#cgo CFLAGS: -I${SRCDIR}/include\n")
	buf.WriteString("#cgo LDFLAGS: -L${SRCDIR}/sdk/lib/release -ldiscord_partner_sdk\n")
	buf.WriteString("#include \"cdiscord.h\"\n#include <stdlib.h>\n\n")
	
	cBuf.WriteString("// Code generated by gen; DO NOT EDIT.\n\n#include \"cdiscord.h\"\n#include <stdlib.h>\n\n")

	uniqueSigs := make(map[string]Callback)
	for _, cb := range g.callbacks {
		if _, ok := uniqueSigs[cb.Signature]; !ok { uniqueSigs[cb.Signature] = cb }
	}

	for sig, cb := range uniqueSigs {
		retC := cb.CReturnType
		if retC == "" { retC = "void" }
		
		decl := fmt.Sprintf("%s goHandle_%s(", retC, sig)
		for i, arg := range cb.Args {
			if i > 0 { decl += ", " }
			decl += fmt.Sprintf("%s %s", arg.CType, arg.Name)
		}
		decl += ");"
		
		buf.WriteString(decl + "\n")
		cBuf.WriteString(decl + "\n")
		
		gatewayDecl := fmt.Sprintf("%s gateway_%s(", retC, sig)
		for i, arg := range cb.Args {
			if i > 0 { gatewayDecl += ", " }
			gatewayDecl += fmt.Sprintf("%s %s", arg.CType, arg.Name)
		}
		gatewayDecl += ")"
		
		buf.WriteString(gatewayDecl + ";\n")
		
		cBuf.WriteString(gatewayDecl + " {\n")
		if retC != "void" { cBuf.WriteString("\treturn ") } else { cBuf.WriteString("\t") }
		cBuf.WriteString(fmt.Sprintf("goHandle_%s(", sig))
		for i, arg := range cb.Args {
			if i > 0 { cBuf.WriteString(", ") }
			cBuf.WriteString(arg.Name)
		}
		cBuf.WriteString(");\n}\n\n")
	}
	
	buf.WriteString("extern void goUnregisterCallback(void* userData);\n")
	buf.WriteString("void gateway_Free(void* userData);\n")
	
	cBuf.WriteString("extern void goUnregisterCallback(void* userData);\n")
	cBuf.WriteString("void gateway_Free(void* userData) {\n\tgoUnregisterCallback(userData);\n}\n")

	buf.WriteString("*/\nimport \"C\"\nimport \"unsafe\"\nimport \"runtime\"\nimport \"sync\"\n\n")

	buf.WriteString("var (\n\tcallbackRegistry = make(map[uintptr]interface{})\n\tcallbackRegistryIdx uintptr\n\tcallbackMu sync.Mutex\n)\n\nfunc registerCallback(cb interface{}) unsafe.Pointer {\n\tcallbackMu.Lock()\n\tdefer callbackMu.Unlock()\n\tcallbackRegistryIdx++\n\tcallbackRegistry[callbackRegistryIdx] = cb\n\treturn unsafe.Pointer(callbackRegistryIdx)\n}\n\n//export goUnregisterCallback\nfunc goUnregisterCallback(userData unsafe.Pointer) {\n\tcallbackMu.Lock()\n\tdefer callbackMu.Unlock()\n\tdelete(callbackRegistry, uintptr(userData))\n}\n\n")

	for sig, cb := range uniqueSigs {
		buf.WriteString(fmt.Sprintf("//export goHandle_%s\nfunc goHandle_%s(", sig, sig))
		for i, arg := range cb.Args {
			if i > 0 { buf.WriteString(", ") }
			goArgType := ""
			if arg.CType == "void*" || arg.CType == "void *" { goArgType = "unsafe.Pointer" } else if strings.HasSuffix(arg.CType, "*") {
				inner := strings.TrimSuffix(arg.CType, "*"); inner = strings.TrimSpace(inner); goArgType = "*C." + inner
			} else { goArgType = "C." + arg.CType }
			buf.WriteString(fmt.Sprintf("%s %s", arg.Name, goArgType))
		}
		buf.WriteString(")")
		if cb.ReturnType != "" {
			retType := "C." + cb.CReturnType
			if cb.CReturnType == "void*" || cb.CReturnType == "void *" { retType = "unsafe.Pointer" }
			buf.WriteString(" " + retType)
		}
		buf.WriteString(" {\n")
		userDataVar := ""
		for _, arg := range cb.Args {
			if strings.Contains(arg.Name, "userData") { userDataVar = arg.Name; break }
		}
		if userDataVar != "" {
			buf.WriteString("\tcallbackMu.Lock()\n")
			buf.WriteString(fmt.Sprintf("\tcbRaw, ok := callbackRegistry[uintptr(unsafe.Pointer(%s))]\n", userDataVar))
			buf.WriteString("\tcallbackMu.Unlock()\n\tif !ok { ")
			if cb.ReturnType != "" { buf.WriteString("return nil") } else { buf.WriteString("return") }
			buf.WriteString(" }\n\tcb := cbRaw.(func(")
			first := true
			for _, arg := range cb.Args {
				if strings.Contains(arg.Name, "userData") { continue }
				if !first { buf.WriteString(", ") }
				first = false; buf.WriteString(arg.Type)
			}
			buf.WriteString(")")
			if cb.ReturnType != "" { buf.WriteString(" " + cb.ReturnType) }
			buf.WriteString(")\n\t")
			if cb.ReturnType != "" { buf.WriteString("res_go := ") }
			buf.WriteString("cb(")
			first = true
			for _, arg := range cb.Args {
				if strings.Contains(arg.Name, "userData") { continue }
				if !first { buf.WriteString(", ") }
				first = false
				if arg.Type == "string" { buf.WriteString(fmt.Sprintf("discordStringToString(%s)", arg.Name)) } else if arg.IsBool { buf.WriteString(fmt.Sprintf("bool(%s)", arg.Name)) } else if g.isKnownStruct(arg.Type) { buf.WriteString(fmt.Sprintf("*(*%s)(unsafe.Pointer(&%s))", arg.Type, arg.Name)) } else if strings.HasPrefix(arg.Type, "*") {
					inner := strings.TrimPrefix(arg.Type, "*"); buf.WriteString(fmt.Sprintf("(*%s)(unsafe.Pointer(%s))", inner, arg.Name))
				} else if strings.HasPrefix(arg.Type, "[]") {
					buf.WriteString(fmt.Sprintf("%sToSlice(%s)", arg.CType, arg.Name))
				} else { buf.WriteString(fmt.Sprintf("%s(%s)", arg.Type, arg.Name)) }
			}
			buf.WriteString(")\n")
			if cb.ReturnType != "" {
				retCast := "C." + cb.CReturnType
				if cb.CReturnType == "void*" || cb.CReturnType == "void *" { retCast = "unsafe.Pointer" }
				buf.WriteString("\treturn (" + retCast + ")(res_go)\n")
			}
		} else if cb.ReturnType != "" {
			retCast := "C." + cb.CReturnType
			if cb.CReturnType == "void*" || cb.CReturnType == "void *" { retCast = "unsafe.Pointer" }
			buf.WriteString("\treturn (" + retCast + ")(nil)\n")
		}
		buf.WriteString("}\n\n")
	}

	buf.WriteString("func stringToDiscordString(s string) C.Discord_String {\n\treturn C.Discord_String{\n\t\tptr: (*C.uint8_t)(unsafe.Pointer(C.CString(s))),\n\t\tsize: C.size_t(len(s)),\n\t}\n}\n\nfunc freeDiscordString(ds C.Discord_String) {\n\tC.free(unsafe.Pointer(ds.ptr))\n}\n\nfunc discordStringToString(ds C.Discord_String) string {\n\tif ds.ptr == nil { return \"\" }\n\treturn C.GoStringN((*C.char)(unsafe.Pointer(ds.ptr)), C.int(ds.size))\n}\n\n")

	enumNames := g.sortedEnumNames()
	for _, name := range enumNames {
		e := g.enums[name]
		if strings.HasSuffix(e.Name, "_forceint") { continue }
		buf.WriteString(fmt.Sprintf("type %s int32\n\nconst (\n", e.Name))
		for _, v := range e.Values {
			if strings.HasSuffix(v.Name, "_forceint") { continue }
			buf.WriteString(fmt.Sprintf("\t%s %s = %d\n", v.Name, e.Name, v.Value))
		}
		buf.WriteString(")\n\n")
	}

	hasInit := make(map[string]bool)
	hasDrop := make(map[string]bool)
	for _, f := range g.functions {
		if strings.HasSuffix(f.Name, "_Init") { hasInit[strings.TrimSuffix(f.Name, "_Init")] = true }
		if strings.HasSuffix(f.Name, "_Drop") { hasDrop[strings.TrimSuffix(f.Name, "_Drop")] = true }
	}

	structNames := g.sortedStructNames()
	for _, name := range structNames {
		s := g.structs[name]
		if ok, elem := g.isSpanType(name); ok {
			buf.WriteString(fmt.Sprintf("func %sToSlice(s C.%s) []%s {\n", name, name, elem))
			buf.WriteString(fmt.Sprintf("\tif s.ptr == nil { return nil }\n"))
			buf.WriteString(fmt.Sprintf("\treturn unsafe.Slice((*%s)(unsafe.Pointer(s.ptr)), int(s.size))\n", elem))
			buf.WriteString("}\n\n")
			buf.WriteString(fmt.Sprintf("func sliceTo%s(s []%s) C.%s {\n", name, elem, name))
			buf.WriteString(fmt.Sprintf("\tif len(s) == 0 { return C.%s{} }\n", name))
			buf.WriteString(fmt.Sprintf("\treturn C.%s{\n", name))
			cElem := elem
			if elem == "uint64" { cElem = "uint64_t" } else if elem == "int32" { cElem = "int32_t" } else if elem == "uint32" { cElem = "uint32_t" } else if elem == "int16" { cElem = "int16_t" } else if elem == "uint8" { cElem = "uint8_t" }
			buf.WriteString(fmt.Sprintf("\t\tptr: (*C.%s)(unsafe.Pointer(&s[0])),\n", cElem))
			buf.WriteString("\t\tsize: C.size_t(len(s)),\n")
			buf.WriteString("\t}\n}\n\n")
			continue
		}
		if s.IsOpaque {
			buf.WriteString(fmt.Sprintf("type %s struct {\n\topaque unsafe.Pointer\n}\n\n", s.Name))
			if hasInit[name] && hasDrop[name] {
				buf.WriteString(fmt.Sprintf("func New%s() *%s {\n\ts := &%s{}\n\ts.Init()\n\truntime.SetFinalizer(s, (*%s).Drop)\n\treturn s\n}\n\n", strings.TrimPrefix(name, "Discord_"), name, name, name))
			}
		} else { buf.WriteString(fmt.Sprintf("type %s C.%s\n\n", s.Name, s.Name)) }
	}

	cbNames := g.sortedCallbackNames()
	for _, name := range cbNames {
		cb := g.callbacks[name]
		buf.WriteString(fmt.Sprintf("// %s is a Go-friendly alias for the callback\ntype %s func(", name, name))
		first := true
		for _, arg := range cb.Args {
			if strings.Contains(arg.Name, "userData") { continue }
			if !first { buf.WriteString(", ") }
			first = false; buf.WriteString(fmt.Sprintf("%s %s", sanitizeGoName(arg.Name), arg.Type))
		}
		buf.WriteString(")")
		if cb.ReturnType != "" { buf.WriteString(" " + cb.ReturnType) }
		buf.WriteString("\n\n")
	}

	for _, f := range g.functions {
		if !strings.HasPrefix(f.Name, "Discord_") { continue }
		goName := strings.TrimPrefix(f.Name, "Discord_")
		isMethod, receiverName, receiverType := false, "", ""
		if len(f.Args) > 0 && strings.HasPrefix(f.Args[0].Type, "*Discord_") {
			isMethod, receiverType = true, f.Args[0].Type
			structName := strings.TrimPrefix(receiverType, "*")
			if strings.HasPrefix(goName, structName[8:]+"_") { goName = strings.TrimPrefix(goName, structName[8:]+"_") }
			receiverName = "self"
		}
		var filteredArgs []Arg
		for _, arg := range f.Args {
			if strings.Contains(arg.Name, "userData") || strings.Contains(arg.Name, "userDataFree") { continue }
			filteredArgs = append(filteredArgs, arg)
		}
		buf.WriteString(fmt.Sprintf("// %s wraps %s\n", goName, f.Name))
		if isMethod {
			buf.WriteString(fmt.Sprintf("func (%s %s) %s(", receiverName, receiverType, goName))
			for i := 1; i < len(filteredArgs); i++ {
				if i > 1 { buf.WriteString(", ") }
				buf.WriteString(fmt.Sprintf("%s %s", sanitizeGoName(filteredArgs[i].Name), filteredArgs[i].Type))
			}
		} else {
			buf.WriteString(fmt.Sprintf("func %s(", goName))
			for i, arg := range filteredArgs {
				if i > 0 { buf.WriteString(", ") }
				buf.WriteString(fmt.Sprintf("%s %s", sanitizeGoName(arg.Name), arg.Type))
			}
		}
		buf.WriteString(")")
		var returnTypes []string
		if f.OutArg != nil { returnTypes = append(returnTypes, f.OutArg.Type) }
		if f.ReturnType != "" { returnTypes = append(returnTypes, f.ReturnType) }
		if len(returnTypes) == 1 { buf.WriteString(" " + returnTypes[0]) } else if len(returnTypes) > 1 { buf.WriteString(" (" + strings.Join(returnTypes, ", ") + ")") }
		buf.WriteString(" {\n")
		if f.OutArg != nil {
			if f.OutArg.Type == "string" { buf.WriteString("\tvar res_out C.Discord_String\n") } else if g.isKnownStruct(f.OutArg.Type) { buf.WriteString(fmt.Sprintf("\tvar res_out C.%s\n", f.OutArg.Type)) } else if strings.HasPrefix(f.OutArg.Type, "[]") {
				buf.WriteString(fmt.Sprintf("\tvar res_out C.%s\n", f.OutArg.CType))
			} else { buf.WriteString(fmt.Sprintf("\tvar res_out C.%s\n", f.OutArg.CType)) }
		}
		var callArgs []string
		for _, arg := range f.Args {
			argVar := sanitizeGoName(arg.Name)
			if isMethod && arg.Name == f.Args[0].Name { argVar = receiverName }
			if strings.Contains(arg.Name, "userDataFree") { callArgs = append(callArgs, "(C.Discord_FreeFn)(unsafe.Pointer(C.gateway_Free))"); continue }
			if strings.Contains(arg.Name, "userData") {
				var targetCb *Arg
				for _, a := range f.Args {
					if cb, ok := g.callbacks[a.Type]; ok {
						targetCb = &a
						buf.WriteString(fmt.Sprintf("\tptr_%s := registerCallback(func(", arg.Name))
						first := true
						for _, cbArg := range cb.Args {
							if strings.Contains(cbArg.Name, "userData") { continue }
							if !first { buf.WriteString(", ") }
							first = false; buf.WriteString(fmt.Sprintf("%s %s", cbArg.Name, cbArg.Type))
						}
						buf.WriteString(")")
						if cb.ReturnType != "" { buf.WriteString(" " + cb.ReturnType) }
						buf.WriteString(fmt.Sprintf(" {\n\t\t"))
						if cb.ReturnType != "" { buf.WriteString("return ") }
						buf.WriteString(fmt.Sprintf("%s(", sanitizeGoName(a.Name)))
						first = true
						for _, cbArg := range cb.Args {
							if strings.Contains(cbArg.Name, "userData") { continue }
							if !first { buf.WriteString(", ") }
							first = false; buf.WriteString(cbArg.Name)
						}
						buf.WriteString(")\n\t})\n")
						callArgs = append(callArgs, fmt.Sprintf("ptr_%s", arg.Name))
						break
					}
				}
				if targetCb == nil { callArgs = append(callArgs, "nil") }
				continue
			}
			if cb, ok := g.callbacks[arg.Type]; ok { callArgs = append(callArgs, fmt.Sprintf("(C.%s)(unsafe.Pointer(C.gateway_%s))", arg.Type, cb.Signature)); continue }
			if arg.Type == "string" {
				if strings.HasSuffix(arg.CType, "*") {
					buf.WriteString(fmt.Sprintf("\tvar p_%s *C.Discord_String\n", argVar))
					buf.WriteString(fmt.Sprintf("\tif %s != \"\" {\n", argVar))
					buf.WriteString(fmt.Sprintf("\t\tc_%s := stringToDiscordString(%s)\n", argVar, argVar))
					buf.WriteString(fmt.Sprintf("\t\tdefer freeDiscordString(c_%s)\n", argVar))
					buf.WriteString(fmt.Sprintf("\t\tp_%s = &c_%s\n", argVar, argVar))
					buf.WriteString("\t}\n")
					callArgs = append(callArgs, fmt.Sprintf("p_%s", argVar))
				} else {
					buf.WriteString(fmt.Sprintf("\tc_%s := stringToDiscordString(%s)\n\tdefer freeDiscordString(c_%s)\n", argVar, argVar, argVar)); callArgs = append(callArgs, fmt.Sprintf("c_%s", argVar))
				}
			} else if strings.HasPrefix(arg.Type, "[]") {
				spanType := arg.CType
				buf.WriteString(fmt.Sprintf("\tc_%s := sliceTo%s(%s)\n", argVar, spanType, argVar))
				callArgs = append(callArgs, fmt.Sprintf("c_%s", argVar))
			} else if arg.IsBool { callArgs = append(callArgs, fmt.Sprintf("C.bool(%s)", argVar)) } else if arg.CType == "void*" || arg.CType == "void *" { callArgs = append(callArgs, fmt.Sprintf("unsafe.Pointer(%s)", argVar)) } else if strings.HasPrefix(arg.Type, "*") {
				cInner := strings.TrimSuffix(arg.CType, "*"); cInner = strings.TrimSpace(cInner); callArgs = append(callArgs, fmt.Sprintf("(*C.%s)(unsafe.Pointer(%s))", cInner, argVar))
			} else if g.isKnownStruct(arg.Type) { callArgs = append(callArgs, fmt.Sprintf("(C.%s)(%s)", arg.Type, argVar)) } else {
				if _, ok := g.enums[arg.Type]; ok { callArgs = append(callArgs, fmt.Sprintf("(C.%s)(%s)", arg.Type, argVar)) } else if arg.Type == "uintptr" && arg.CType == "size_t" { callArgs = append(callArgs, fmt.Sprintf("C.size_t(%s)", argVar)) } else { callArgs = append(callArgs, fmt.Sprintf("(C.%s)(%s)", arg.CType, argVar)) }
			}
		}
		if f.OutArg != nil { callArgs = append(callArgs, "&res_out") }
		callStr := fmt.Sprintf("C.%s(%s)", f.Name, strings.Join(callArgs, ", "))
		if f.ReturnType == "" { buf.WriteString("\t" + callStr + "\n") } else { buf.WriteString("\tres_c := " + callStr + "\n") }
		var retVars []string
		if f.OutArg != nil {
			if f.OutArg.Type == "string" { retVars = append(retVars, "discordStringToString(res_out)") } else if f.OutArg.IsBool { retVars = append(retVars, "bool(res_out)") } else if strings.HasPrefix(f.OutArg.Type, "[]") {
				retVars = append(retVars, fmt.Sprintf("%sToSlice(res_out)", f.OutArg.CType))
			} else if s, ok := g.structs[f.OutArg.Type]; ok && s.IsOpaque { retVars = append(retVars, fmt.Sprintf("*(*%s)(unsafe.Pointer(&res_out))", f.OutArg.Type)) } else { retVars = append(retVars, fmt.Sprintf("%s(res_out)", f.OutArg.Type)) }
		}
		if f.ReturnType != "" {
			if f.ReturnType == "unsafe.Pointer" { retVars = append(retVars, "unsafe.Pointer(res_c)") } else if f.ReturnType == "string" { retVars = append(retVars, "discordStringToString(res_c)") } else if f.ReturnIsBool { retVars = append(retVars, "bool(res_c)") } else if strings.HasPrefix(f.ReturnType, "[]") {
				retVars = append(retVars, fmt.Sprintf("%sToSlice(res_c)", f.CReturnType))
			} else if s, ok := g.structs[f.ReturnType]; ok && s.IsOpaque { retVars = append(retVars, fmt.Sprintf("*(*%s)(unsafe.Pointer(&res_c))", f.ReturnType)) } else if strings.HasPrefix(f.ReturnType, "*") {
				inner := strings.TrimPrefix(f.ReturnType, "*"); retVars = append(retVars, fmt.Sprintf("(*%s)(unsafe.Pointer(res_c))", inner))
			} else { retVars = append(retVars, fmt.Sprintf("%s(res_c)", f.ReturnType)) }
		}
		if len(retVars) > 0 { buf.WriteString("\treturn " + strings.Join(retVars, ", ") + "\n") }
		buf.WriteString("}\n\n")
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil { fmt.Printf("Warning: could not format source: %v\n", err); os.WriteFile("discord.go.err", buf.Bytes(), 0644); return }
	err = os.WriteFile("discord.go", formatted, 0644); if err != nil { log.Fatalf("Failed to write discord.go: %v", err) }
	fmt.Println("Generated discord.go")
	err = os.WriteFile("gateway.c", cBuf.Bytes(), 0644); if err != nil { log.Fatalf("Failed to write gateway.c: %v", err) }
	fmt.Println("Generated gateway.c")
}

func (g *Generator) isKnownStruct(name string) bool { _, ok := g.structs[name]; return ok }
func (g *Generator) sortedEnumNames() []string { var names []string; for n := range g.enums { names = append(names, n) }; sort.Strings(names); return names }
func (g *Generator) sortedStructNames() []string { var names []string; for n := range g.structs { names = append(names, n) }; sort.Strings(names); return names }
func (g *Generator) sortedCallbackNames() []string { var names []string; for n := range g.callbacks { names = append(names, n) }; sort.Strings(names); return names }
