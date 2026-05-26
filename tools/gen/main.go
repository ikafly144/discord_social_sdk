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

	OutputFile string
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
	Name       string
	ReturnType string
	Args       []Arg
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
	if len(os.Args) < 3 {
		log.Fatal("Usage: gen <header_file> <output_file>")
	}
	headerFile := os.Args[1]
	outputFile := os.Args[2]

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
		enums:      make(map[string]Enum),
		structs:    make(map[string]Struct),
		callbacks:  make(map[string]Callback),
		OutputFile: outputFile,
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
			// Pull it out as an out-parameter
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

	cb := Callback{
		Name:       name,
		ReturnType: getString(C.clang_getTypeSpelling(C.clang_getResultType(proto))),
	}

	numArgs := int(C.clang_getNumArgTypes(proto))
	for i := 0; i < numArgs; i++ {
		argType := C.clang_getArgType(proto, C.uint(i))
		cb.Args = append(cb.Args, Arg{
			Name: fmt.Sprintf("arg%d", i),
			Type: getString(C.clang_getTypeSpelling(argType)),
		})
	}
	g.callbacks[name] = cb
}

func mapCTypeToGo(t C.CXType) (goType, cType string, isBool bool) {
	kind := t.kind
	spelling := getString(C.clang_getTypeSpelling(t))

	// Helper to strip const
	stripConst := func(s string) string {
		s = strings.TrimPrefix(s, "const ")
		s = strings.TrimSuffix(s, " const")
		return s
	}

	cType = stripConst(spelling)
	canonical := C.clang_getCanonicalType(t)

	if cType == "bool" || cType == "_Bool" || canonical.kind == C.CXType_Bool {
		return "bool", "bool", true
	}

	goType = cType

	switch kind {
	case C.CXType_Bool:
		return "bool", "bool", true
	case C.CXType_Int, C.CXType_SChar, C.CXType_Short, C.CXType_Long, C.CXType_LongLong:
		if cType == "int8_t" {
			return "int8", "int8_t", false
		}
		if cType == "int16_t" {
			return "int16", "int16_t", false
		}
		if cType == "int32_t" {
			return "int32", "int32_t", false
		}
		if cType == "int64_t" {
			return "int64", "int64_t", false
		}
		return "int", cType, false
	case C.CXType_UInt, C.CXType_UChar, C.CXType_UShort, C.CXType_ULong, C.CXType_ULongLong:
		if cType == "uint8_t" {
			return "uint8", "uint8_t", false
		}
		if cType == "uint16_t" {
			return "uint16", "uint16_t", false
		}
		if cType == "uint32_t" {
			return "uint32", "uint32_t", false
		}
		if cType == "uint64_t" {
			return "uint64", "uint64_t", false
		}
		return "uint", cType, false
	case C.CXType_Float:
		return "float32", "float", false
	case C.CXType_Double:
		return "float64", "double", false
	case C.CXType_Pointer:
		pointee := C.clang_getPointeeType(t)
		if pointee.kind == C.CXType_Void {
			return "unsafe.Pointer", "void*", false
		}
		goPointee, cPointee, _ := mapCTypeToGo(pointee)
		return "*" + goPointee, cPointee + "*", false
	case C.CXType_Void:
		return "", "void", false
	case C.CXType_Typedef:
		if cType == "Discord_String" {
			return "string", "Discord_String", false
		}
		if cType == "size_t" {
			return "uintptr", "size_t", false
		}
		if cType == "uint64_t" {
			return "uint64", "uint64_t", false
		}
		if cType == "int32_t" {
			return "int32", "int32_t", false
		}
		if cType == "uint32_t" {
			return "uint32", "uint32_t", false
		}
		if cType == "uint8_t" {
			return "uint8", "uint8_t", false
		}
		if cType == "int16_t" {
			return "int16", "int16_t", false
		}
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
	if goKeywords[name] {
		return name + "_"
	}
	return name
}

func (g *Generator) generate() {
	var buf bytes.Buffer

	buf.WriteString("// Code generated by gen; DO NOT EDIT.\n\n")
	buf.WriteString("package discord\n\n")
	buf.WriteString("/*\n")
	buf.WriteString("#cgo CFLAGS: -Iinclude\n")
	buf.WriteString("#include \"cdiscord.h\"\n")
	buf.WriteString("#include <stdlib.h>\n")
	buf.WriteString("*/\n")
	buf.WriteString("import \"C\"\n")
	buf.WriteString("import \"unsafe\"\n\n")

	// Helper functions for string conversion
	buf.WriteString("func stringToDiscordString(s string) C.Discord_String {\n")
	buf.WriteString("\treturn C.Discord_String{\n")
	buf.WriteString("\t\tptr: (*C.uint8_t)(unsafe.Pointer(C.CString(s))),\n")
	buf.WriteString("\t\tsize: C.size_t(len(s)),\n")
	buf.WriteString("\t}\n")
	buf.WriteString("}\n\n")

	buf.WriteString("func freeDiscordString(ds C.Discord_String) {\n")
	buf.WriteString("\tC.free(unsafe.Pointer(ds.ptr))\n")
	buf.WriteString("}\n\n")

	buf.WriteString("func discordStringToString(ds C.Discord_String) string {\n")
	buf.WriteString("\tif ds.ptr == nil { return \"\" }\n")
	buf.WriteString("\treturn C.GoStringN((*C.char)(unsafe.Pointer(ds.ptr)), C.int(ds.size))\n")
	buf.WriteString("}\n\n")

	// Generate Enums
	enumNames := g.sortedEnumNames()
	for _, name := range enumNames {
		e := g.enums[name]
		if strings.HasSuffix(e.Name, "_forceint") {
			continue
		}
		buf.WriteString(fmt.Sprintf("type %s int32\n\n", e.Name))
		buf.WriteString("const (\n")
		for _, v := range e.Values {
			if strings.HasSuffix(v.Name, "_forceint") {
				continue
			}
			buf.WriteString(fmt.Sprintf("\t%s %s = %d\n", v.Name, e.Name, v.Value))
		}
		buf.WriteString(")\n\n")
	}

	// Generate Structs
	structNames := g.sortedStructNames()
	for _, name := range structNames {
		s := g.structs[name]
		if s.IsOpaque {
			buf.WriteString(fmt.Sprintf("type %s struct {\n", s.Name))
			buf.WriteString("\topaque unsafe.Pointer\n")
			buf.WriteString("}\n\n")
		} else {
			buf.WriteString(fmt.Sprintf("type %s C.%s\n\n", s.Name, s.Name))
		}
	}

	// Generate Callbacks
	cbNames := g.sortedCallbackNames()
	for _, name := range cbNames {
		buf.WriteString(fmt.Sprintf("type %s unsafe.Pointer\n\n", name))
	}

	// Generate Functions and Methods
	for _, f := range g.functions {
		if !strings.HasPrefix(f.Name, "Discord_") {
			continue
		}

		goName := strings.TrimPrefix(f.Name, "Discord_")
		isMethod := false
		receiverName := ""
		receiverType := ""

		if len(f.Args) > 0 && strings.HasPrefix(f.Args[0].Type, "*Discord_") {
			isMethod = true
			receiverType = f.Args[0].Type
			structName := strings.TrimPrefix(receiverType, "*")
			if strings.HasPrefix(goName, structName[8:]+"_") { // 8 is len("Discord_")
				goName = strings.TrimPrefix(goName, structName[8:]+"_")
			}
			receiverName = "self"
		}

		buf.WriteString(fmt.Sprintf("// %s wraps %s\n", goName, f.Name))
		if isMethod {
			buf.WriteString(fmt.Sprintf("func (%s %s) %s(", receiverName, receiverType, goName))
			for i := 1; i < len(f.Args); i++ {
				arg := f.Args[i]
				if i > 1 {
					buf.WriteString(", ")
				}
				buf.WriteString(fmt.Sprintf("%s %s", sanitizeGoName(arg.Name), arg.Type))
			}
		} else {
			buf.WriteString(fmt.Sprintf("func %s(", goName))
			for i, arg := range f.Args {
				if i > 0 {
					buf.WriteString(", ")
				}
				buf.WriteString(fmt.Sprintf("%s %s", sanitizeGoName(arg.Name), arg.Type))
			}
		}
		buf.WriteString(")")

		var returnTypes []string
		if f.OutArg != nil {
			returnTypes = append(returnTypes, f.OutArg.Type)
		}
		if f.ReturnType != "" {
			returnTypes = append(returnTypes, f.ReturnType)
		}

		if len(returnTypes) == 1 {
			buf.WriteString(" " + returnTypes[0])
		} else if len(returnTypes) > 1 {
			buf.WriteString(" (" + strings.Join(returnTypes, ", ") + ")")
		}

		buf.WriteString(" {\n")

		// Setup out variable if exists
		if f.OutArg != nil {
			if f.OutArg.Type == "string" {
				buf.WriteString("\tvar res_out C.Discord_String\n")
			} else if g.isKnownStruct(f.OutArg.Type) {
				buf.WriteString(fmt.Sprintf("\tvar res_out C.%s\n", f.OutArg.Type))
			} else {
				buf.WriteString(fmt.Sprintf("\tvar res_out C.%s\n", f.OutArg.CType))
			}
		}

		// Setup arguments and call
		var callArgs []string
		for i, arg := range f.Args {
			argVar := sanitizeGoName(arg.Name)
			if isMethod && i == 0 {
				argVar = receiverName
			}

			if arg.Type == "string" {
				buf.WriteString(fmt.Sprintf("\tc_%s := stringToDiscordString(%s)\n", argVar, argVar))
				buf.WriteString(fmt.Sprintf("\tdefer freeDiscordString(c_%s)\n", argVar))
				callArgs = append(callArgs, fmt.Sprintf("c_%s", argVar))
			} else if arg.IsBool {
				callArgs = append(callArgs, fmt.Sprintf("C.bool(%s)", argVar))
			} else if arg.CType == "void*" || arg.CType == "void *" {
				callArgs = append(callArgs, fmt.Sprintf("unsafe.Pointer(%s)", argVar))
			} else if strings.HasPrefix(arg.Type, "*") {
				cInner := strings.TrimSuffix(arg.CType, "*")
				cInner = strings.TrimSpace(cInner)
				callArgs = append(callArgs, fmt.Sprintf("(*C.%s)(unsafe.Pointer(%s))", cInner, argVar))
			} else if g.isKnownStruct(arg.Type) {
				callArgs = append(callArgs, fmt.Sprintf("(C.%s)(%s)", arg.Type, argVar))
			} else {
				// Check if it's an enum
				if _, ok := g.enums[arg.Type]; ok {
					callArgs = append(callArgs, fmt.Sprintf("(C.%s)(%s)", arg.Type, argVar))
				} else if arg.Type == "uintptr" && arg.CType == "size_t" {
					callArgs = append(callArgs, fmt.Sprintf("C.size_t(%s)", argVar))
				} else {
					callArgs = append(callArgs, fmt.Sprintf("(C.%s)(%s)", arg.CType, argVar))
				}
			}
		}
		if f.OutArg != nil {
			callArgs = append(callArgs, "&res_out")
		}

		callStr := fmt.Sprintf("C.%s(%s)", f.Name, strings.Join(callArgs, ", "))
		if f.ReturnType == "" {
			buf.WriteString("\t" + callStr + "\n")
		} else {
			buf.WriteString("\tres_c := " + callStr + "\n")
		}

		// Handle returns
		var retVars []string
		if f.OutArg != nil {
			if f.OutArg.Type == "string" {
				retVars = append(retVars, "discordStringToString(res_out)")
			} else if f.OutArg.IsBool {
				retVars = append(retVars, "bool(res_out)")
			} else if s, ok := g.structs[f.OutArg.Type]; ok && s.IsOpaque {
				retVars = append(retVars, fmt.Sprintf("*(*%s)(unsafe.Pointer(&res_out))", f.OutArg.Type))
			} else {
				retVars = append(retVars, fmt.Sprintf("%s(res_out)", f.OutArg.Type))
			}
		}
		if f.ReturnType != "" {
			if f.ReturnType == "unsafe.Pointer" {
				retVars = append(retVars, "unsafe.Pointer(res_c)")
			} else if f.ReturnType == "string" {
				retVars = append(retVars, "discordStringToString(res_c)")
			} else if f.ReturnIsBool {
				retVars = append(retVars, "bool(res_c)")
			} else if s, ok := g.structs[f.ReturnType]; ok && s.IsOpaque {
				retVars = append(retVars, fmt.Sprintf("*(*%s)(unsafe.Pointer(&res_c))", f.ReturnType))
			} else if strings.HasPrefix(f.ReturnType, "*") {
				inner := strings.TrimPrefix(f.ReturnType, "*")
				retVars = append(retVars, fmt.Sprintf("(*%s)(unsafe.Pointer(res_c))", inner))
			} else {
				retVars = append(retVars, fmt.Sprintf("%s(res_c)", f.ReturnType))
			}
		}

		if len(retVars) > 0 {
			buf.WriteString("\treturn " + strings.Join(retVars, ", ") + "\n")
		}

		buf.WriteString("}\n\n")
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Printf("Warning: could not format source: %v\n", err)
		os.WriteFile(g.OutputFile+".err", buf.Bytes(), 0644)
		return
	}

	err = os.WriteFile(g.OutputFile, formatted, 0644)
	if err != nil {
		log.Fatalf("Failed to write %s: %v", g.OutputFile, err)
	}
	fmt.Printf("Generated %s\n", g.OutputFile)
}

func (g *Generator) isKnownStruct(name string) bool {
	_, ok := g.structs[name]
	return ok
}

func (g *Generator) sortedEnumNames() []string {
	var names []string
	for n := range g.enums {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (g *Generator) sortedStructNames() []string {
	var names []string
	for n := range g.structs {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (g *Generator) sortedCallbackNames() []string {
	var names []string
	for n := range g.callbacks {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}
