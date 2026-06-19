package main

/*
#cgo LDFLAGS: -lclang
#include <clang-c/Index.h>
#include <stdlib.h>

enum CXChildVisitResult visitCursor(CXCursor cursor, CXCursor parent, CXClientData client_data);
enum CXChildVisitResult visitCursorEnum(CXCursor cursor, CXCursor parent, CXClientData client_data);
enum CXChildVisitResult visitCursorStruct(CXCursor cursor, CXCursor parent, CXClientData client_data);
enum CXChildVisitResult visitCallbackParams(CXCursor cursor, CXCursor parent, CXClientData client_data);
*/
import "C"

import (
	"bytes"
	"fmt"
	"go/format"
	"log"
	"maps"
	"os"
	"runtime"
	"slices"
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
	Type string // C type
}

type Function struct {
	Name          string
	ReturnType    string
	CReturnType   string
	ReturnIsBool  bool
	Args          []Arg
	OutArg        *Arg
	ReturnsStruct bool
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
	Signature   string
}

var currentGenerator *Generator
var currentEnum *Enum
var currentStruct *Struct
var currentParamNames []string

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

//export goVisitCallbackParams
func goVisitCallbackParams(c, p *C.CXCursor, clientData unsafe.Pointer) C.enum_CXChildVisitResult {
	if C.clang_getCursorKind(*c) == C.CXCursor_ParmDecl {
		currentParamNames = append(currentParamNames, getString(C.clang_getCursorSpelling(*c)))
	}
	return C.CXChildVisit_Continue
}

func (g *Generator) parseFunction(cursor C.CXCursor) {
	name := getString(C.clang_getCursorSpelling(cursor))
	resType := C.clang_getCursorResultType(cursor)
	retType, cRetType, retIsBool := mapCTypeToGo(resType)

	f := Function{
		Name:         name,
		ReturnType:   retType,
		CReturnType:  cRetType,
		ReturnIsBool: retIsBool,
	}

	if resType.kind == C.CXType_Record || resType.kind == C.CXType_Typedef {
		if retType == "Discord_String" || g.isKnownStruct(retType) {
			f.ReturnsStruct = true
		}
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
	currentParamNames = nil
	C.clang_visitChildren(cursor, C.CXCursorVisitor(C.visitCallbackParams), nil)
	paramNames := currentParamNames
	currentParamNames = nil

	var sigParts []string
	for i := 0; i < numArgs; i++ {
		argType := C.clang_getArgType(proto, C.uint(i))
		goType, cType, isBool := mapCTypeToGo(argType)

		argName := ""
		if i < len(paramNames) {
			argName = paramNames[i]
		}
		if argName == "" {
			argName = fmt.Sprintf("arg%d", i)
		}

		if i == numArgs-1 && (cType == "void*" || cType == "void *") && (argName == "" || strings.Contains(argName, "arg")) {
			argName = "userData"
		}

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
	if !strings.HasSuffix(name, "Span") {
		return false, ""
	}
	s, ok := g.structs[name]
	if !ok || len(s.Fields) != 2 {
		return false, ""
	}
	var hasPtr, hasSize bool
	var elemType string
	for _, f := range s.Fields {
		if f.Name == "ptr" {
			hasPtr = true
			elemType = strings.TrimSuffix(strings.TrimSpace(strings.TrimPrefix(f.Type, "struct ")), "*")
			elemType = strings.TrimSpace(elemType)
		}
		if f.Name == "size" {
			hasSize = true
		}
	}
	if hasPtr && hasSize {
		goElem := elemType
		if t, ok := mapSimpleCTypeToGo(elemType); ok {
			goElem = t
		}
		return true, goElem
	}
	return false, ""
}

func mapSimpleCTypeToGo(cType string) (string, bool) {
	cType = strings.TrimPrefix(cType, "const ")
	cType = strings.TrimSuffix(cType, " const")
	switch cType {
	case "uint64_t":
		return "uint64", true
	case "int32_t":
		return "int32", true
	case "uint32_t":
		return "uint32", true
	case "int16_t":
		return "int16", true
	case "uint8_t":
		return "uint8", true
	case "float":
		return "float32", true
	case "double":
		return "float64", true
	case "bool", "_Bool":
		return "bool", true
	case "void*":
		return "unsafe.Pointer", true
	case "size_t":
		return "uintptr", true
	case "Discord_String":
		return "string", true
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
		if ok, elem := currentGenerator.isSpanType(cType); ok {
			return "[]" + elem, cType, false
		}
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
		if goPointee == "string" {
			return "string", cPointee + "*", false
		}
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
	if name == "" {
		return ""
	}
	parts := strings.Split(name, "_")
	var res string
	for _, p := range parts {
		if p == "" {
			continue
		}
		res += strings.ToUpper(p[:1]) + p[1:]
	}
	if res == "" {
		return ""
	}
	if goKeywords[res] {
		return res + "_"
	}
	return res
}

func sanitizeGoNamePrivate(name string) string {
	res := sanitizeGoName(name)
	if res == "" {
		return ""
	}
	res = strings.ToLower(res[:1]) + res[1:]
	if goKeywords[res] {
		return res + "_"
	}
	return res
}

func stripDiscordPrefix(name string) string {
	return strings.TrimPrefix(name, "Discord_")
}

func toGoType(typeName string) string {
	if strings.HasPrefix(typeName, "*") {
		return "*" + toGoType(strings.TrimPrefix(typeName, "*"))
	}
	if strings.HasPrefix(typeName, "[]") {
		return "[]" + toGoType(strings.TrimPrefix(typeName, "[]"))
	}

	stripped := stripDiscordPrefix(typeName)
	switch stripped {
	case "uintptr", "unsafe.Pointer", "string", "bool", "int", "uint", "int8", "uint8", "int16", "uint16", "int32", "uint32", "int64", "uint64", "float32", "float64":
		return stripped
	}

	return sanitizeGoName(stripped)
}

func (g *Generator) mapCTypeStringToGo(cType string) string {
	cType = strings.TrimPrefix(cType, "const ")
	cType = strings.TrimSuffix(cType, " const")
	if ok, elem := g.isSpanType(cType); ok {
		return "[]" + elem
	}
	if t, ok := mapSimpleCTypeToGo(cType); ok {
		return t
	}
	if strings.HasSuffix(cType, "*") {
		pointee := strings.TrimSpace(strings.TrimSuffix(cType, "*"))
		if pointee == "void" {
			return "unsafe.Pointer"
		}
		return "*" + g.mapCTypeStringToGo(pointee)
	}
	if strings.HasPrefix(cType, "struct ") {
		return strings.TrimPrefix(cType, "struct ")
	}
	if strings.HasPrefix(cType, "enum ") {
		return strings.TrimPrefix(cType, "enum ")
	}
	return cType
}

func (g *Generator) mapToFFIType(goType, cType string) string {
	if goType == "string" {
		return "uintptr"
	}
	if strings.HasPrefix(goType, "*") {
		return "uintptr"
	}
	if g.isKnownStruct(goType) || strings.HasSuffix(cType, "Span") {
		return "uintptr"
	}
	if _, ok := g.callbacks[goType]; ok {
		return "uintptr"
	}
	if goType == "bool" {
		return "uintptr"
	}
	if goType == "unsafe.Pointer" {
		return "uintptr"
	}
	if strings.HasSuffix(goType, "Fn") || strings.HasSuffix(goType, "Callback") {
		return "uintptr"
	}
	return goType
}

func (g *Generator) generate() {
	var buf bytes.Buffer
	buf.WriteString("// Code generated by gen; DO NOT EDIT.\n\npackage discord\n\n")
	buf.WriteString("import \"unsafe\"\nimport \"runtime\"\nimport \"sync\"\nimport \"github.com/ebitengine/purego\"\nimport \"path/filepath\"\nimport \"os\"\nimport \"fmt\"\n\n")

	buf.WriteString("var (\n\tlibHandle uintptr\n\tcallbackRegistry = make(map[uintptr]interface{})\n\tcallbackRegistryIdx uintptr\n\tcallbackMu sync.Mutex\n)\n\n")

	buf.WriteString("func init() {\n\tlib, err := loadLibrary()\n\tif err != nil {\n\t\tpanic(err)\n\t}\n\tlibHandle = lib\n\tregisterFunctions()\n}\n\n")

	buf.WriteString("func loadLibrary() (uintptr, error) {\n")
	buf.WriteString("\tvar name string\n\tswitch runtime.GOOS {\n")
	buf.WriteString("\tcase \"windows\":\n\t\tname = \"discord_partner_sdk.dll\"\n")
	buf.WriteString("\tcase \"darwin\":\n\t\tname = \"libdiscord_partner_sdk.dylib\"\n")
	buf.WriteString("\tdefault:\n\t\tname = \"libdiscord_partner_sdk.so\"\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\t// Try to find it in sdk/lib/release\n")
	buf.WriteString("\texe, _ := os.Executable()\n")
	buf.WriteString("\tpaths := []string{\n")
	buf.WriteString("\t\tname,\n")
	buf.WriteString("\t\tfilepath.Join(filepath.Dir(exe), name),\n")
	buf.WriteString("\t\tfilepath.Join(filepath.Dir(exe), \"sdk\", \"lib\", \"release\", name),\n")
	buf.WriteString("\t\tfilepath.Join(\"sdk\", \"lib\", \"release\", name),\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\tfor _, path := range paths {\n")
	buf.WriteString("\t\th, err := openLibraryInternal(path)\n")
	buf.WriteString("\t\tif err == nil {\n\t\t\treturn h, nil\n\t\t}\n")
	buf.WriteString("\t}\n")
	buf.WriteString("\treturn 0, fmt.Errorf(\"could not load library %s\", name)\n")
	buf.WriteString("}\n\n")

	buf.WriteString("var (\n")
	for _, f := range g.functions {
		fmt.Fprintf(&buf, "\t_%s func(", f.Name)
		first := true
		if f.ReturnsStruct && runtime.GOARCH == "amd64" {
			buf.WriteString("uintptr") // The out-pointer for struct return
			first = false
		}
		for _, arg := range f.Args {
			if !first {
				buf.WriteString(", ")
			}
			first = false
			t := g.mapToFFIType(arg.Type, arg.CType)
			buf.WriteString(toGoType(t))
		}
		if f.OutArg != nil {
			if !first {
				buf.WriteString(", ")
			}
			buf.WriteString("uintptr")
		}
		buf.WriteString(")")
		if f.ReturnType != "" && !f.ReturnsStruct {
			buf.WriteString(" ")
			buf.WriteString(toGoType(g.mapToFFIType(f.ReturnType, f.CReturnType)))
		}
		buf.WriteString("\n")
	}
	buf.WriteString(")\n\n")

	buf.WriteString("func registerFunctions() {\n")
	for _, f := range g.functions {
		fmt.Fprintf(&buf, "\tpurego.RegisterLibFunc(&_%s, libHandle, %q)\n", f.Name, f.Name)
	}
	buf.WriteString("}\n\n")

	uniqueSigs := make(map[string]Callback)
	for _, cb := range g.callbacks {
		if _, ok := uniqueSigs[cb.Signature]; !ok {
			uniqueSigs[cb.Signature] = cb
		}
	}

	// Add anonymous inline callbacks used in function arguments
	for _, f := range g.functions {
		for _, arg := range f.Args {
			if strings.HasSuffix(arg.Type, "Callback") || strings.HasSuffix(arg.Type, "Fn") {
				continue // handled by g.callbacks
			}
			if arg.CType == "Discord_FreeFn" || arg.Type == "unsafe.Pointer" {
				continue
			}
		}
	}
	// Manually ensure gateway_voidPtr is generated, it's used for generic free callbacks etc.
	if _, ok := uniqueSigs["voidPtr"]; !ok {
		uniqueSigs["voidPtr"] = Callback{
			Name:      "voidPtr",
			Args:      []Arg{{Name: "userData", Type: "unsafe.Pointer", CType: "void*"}},
			Signature: "voidPtr",
		}
	}

	buf.WriteString("func registerCallback(cb interface{}) unsafe.Pointer {\n\tcallbackMu.Lock()\n\tdefer callbackMu.Unlock()\n\tcallbackRegistryIdx++\n\tcallbackRegistry[callbackRegistryIdx] = cb\n\treturn unsafe.Pointer(callbackRegistryIdx)\n}\n\n")

	generatedGateways := make(map[string]bool)

	for _, name := range g.sortedCallbackNames() {
		cb := g.callbacks[name]
		if userDataVar := -1; true {
			for i, arg := range cb.Args {
				if strings.Contains(arg.Name, "userData") {
					userDataVar = i
					break
				}
			}
			if userDataVar == -1 {
				continue
			}
			gatewayName := "gateway_" + cb.Name
			generatedGateways[gatewayName] = true
			fmt.Fprintf(&buf, "func %s(", gatewayName)
			for i := range cb.Args {
				if i > 0 {
					buf.WriteString(", ")
				}
				fmt.Fprintf(&buf, "arg%d uintptr", i)
			}
			buf.WriteString(")")
			buf.WriteString(" uintptr")
			buf.WriteString(" {\n")
			buf.WriteString("\tcallbackMu.Lock()\n")
			fmt.Fprintf(&buf, "\tcbRaw, ok := callbackRegistry[arg%d]\n", userDataVar)
			buf.WriteString("\tcallbackMu.Unlock()\n")
			buf.WriteString("\tif !ok { return 0 }\n")

			fmt.Fprintf(&buf, "\tcb := cbRaw.(%s)\n", toGoType(cb.Name))

			if cb.ReturnType != "" {
				buf.WriteString("\tres_go := ")
			} else {
				buf.WriteString("\t")
			}
			buf.WriteString("cb(")
			first := true
			for i, arg := range cb.Args {
				if i == userDataVar {
					continue
				}
				if !first {
					buf.WriteString(", ")
				}
				first = false
				if arg.Type == "string" {
					fmt.Fprintf(&buf, "discordStringToString((*String)(unsafe.Pointer(arg%d)))", i)
				} else if g.isKnownStruct(arg.Type) || strings.HasSuffix(arg.CType, "Span") {
					goArgType := toGoType(arg.Type)
					fmt.Fprintf(&buf, "*(*%s)(unsafe.Pointer(arg%d))", goArgType, i)
				} else if strings.HasPrefix(arg.Type, "*") {
					goArgType := toGoType(arg.Type)
					fmt.Fprintf(&buf, "(%s)(unsafe.Pointer(arg%d))", goArgType, i)
				} else if arg.IsBool {
					fmt.Fprintf(&buf, "arg%d != 0", i)
				} else {
					goArgType := toGoType(arg.Type)
					fmt.Fprintf(&buf, "%s(arg%d)", goArgType, i)
				}
			}
			buf.WriteString(")\n")
			if cb.ReturnType != "" {
				buf.WriteString("\treturn uintptr(res_go)\n")
			} else {
				buf.WriteString("\treturn 0\n")
			}
			buf.WriteString("}\n\n")
		}
	}

	for _, sig := range slices.Sorted(maps.Keys(uniqueSigs)) {
		cb := uniqueSigs[sig]
		if userDataVar := -1; true {
			for i, arg := range cb.Args {
				if strings.Contains(arg.Name, "userData") {
					userDataVar = i
					break
				}
			}
			if userDataVar == -1 {
				continue
			}
			gatewayName := "gateway_" + sig
			if generatedGateways[gatewayName] {
				continue
			}
			generatedGateways[gatewayName] = true
			fmt.Fprintf(&buf, "func %s(", gatewayName)
			for i := range cb.Args {
				if i > 0 {
					buf.WriteString(", ")
				}
				fmt.Fprintf(&buf, "arg%d uintptr", i)
			}
			buf.WriteString(")")
			buf.WriteString(" uintptr")
			buf.WriteString(" {\n")
			buf.WriteString("\tcallbackMu.Lock()\n")
			fmt.Fprintf(&buf, "\tcbRaw, ok := callbackRegistry[arg%d]\n", userDataVar)
			buf.WriteString("\tcallbackMu.Unlock()\n")
			buf.WriteString("\tif !ok { return 0 }\n")

			// Build the anonymous function type string
			buf.WriteString("\tcb := cbRaw.(func(")
			first := true
			for i, arg := range cb.Args {
				if i == userDataVar {
					continue
				}
				if !first {
					buf.WriteString(", ")
				}
				first = false
				goArgType := toGoType(arg.Type)
				buf.WriteString(goArgType)
			}
			buf.WriteString(")")
			if cb.ReturnType != "" {
				goRetType := toGoType(cb.ReturnType)
				buf.WriteString(" ")
				buf.WriteString(goRetType)
			}
			buf.WriteString(")\n")

			if cb.ReturnType != "" {
				buf.WriteString("\tres_go := ")
			} else {
				buf.WriteString("\t")
			}
			buf.WriteString("cb(")
			first = true
			for i, arg := range cb.Args {
				if i == userDataVar {
					continue
				}
				if !first {
					buf.WriteString(", ")
				}
				first = false
				if arg.Type == "string" {
					fmt.Fprintf(&buf, "discordStringToString((*String)(unsafe.Pointer(arg%d)))", i)
				} else if g.isKnownStruct(arg.Type) || strings.HasSuffix(arg.CType, "Span") {
					goArgType := toGoType(arg.Type)
					fmt.Fprintf(&buf, "*(*%s)(unsafe.Pointer(arg%d))", goArgType, i)
				} else if strings.HasPrefix(arg.Type, "*") {
					goArgType := toGoType(arg.Type)
					fmt.Fprintf(&buf, "(%s)(unsafe.Pointer(arg%d))", goArgType, i)
				} else if arg.IsBool {
					fmt.Fprintf(&buf, "arg%d != 0", i)
				} else {
					goArgType := toGoType(arg.Type)
					fmt.Fprintf(&buf, "%s(arg%d)", goArgType, i)
				}
			}
			buf.WriteString(")\n")
			if cb.ReturnType != "" {
				buf.WriteString("\treturn uintptr(res_go)\n")
			} else {
				buf.WriteString("\treturn 0\n")
			}
			buf.WriteString("}\n\n")
		}
	}

	buf.WriteString("func discordStringToString(ds *String) string {\n\tif ds == nil || ds.ptr == nil { return \"\" }\n\treturn string(unsafe.Slice(ds.ptr, int(ds.size)))\n}\n\n")
	buf.WriteString("func stringToDiscordString(s string) String {\n\treturn String{\n\t\tptr: (*byte)(unsafe.Pointer(unsafe.StringData(s))),\n\t\tsize: uintptr(len(s)),\n\t}\n}\n\n")

	enumNames := g.sortedEnumNames()
	for _, name := range enumNames {
		e := g.enums[name]
		if strings.HasSuffix(e.Name, "_forceint") {
			continue
		}
		goEnumName := toGoType(e.Name)
		fmt.Fprintf(&buf, "type %s int32\n\nconst (\n", goEnumName)
		for _, v := range e.Values {
			if strings.HasSuffix(v.Name, "_forceint") {
				continue
			}
			goValName := toGoType(v.Name)
			fmt.Fprintf(&buf, "\t%s %s = %d\n", goValName, goEnumName, v.Value)
		}
		buf.WriteString(")\n\n")
	}

	hasInit := make(map[string]bool)
	hasDrop := make(map[string]bool)
	for _, f := range g.functions {
		if strings.HasSuffix(f.Name, "_Init") {
			hasInit[strings.TrimSuffix(f.Name, "_Init")] = true
		}
		if strings.HasSuffix(f.Name, "_Drop") {
			hasDrop[strings.TrimSuffix(f.Name, "_Drop")] = true
		}
	}

	structNames := g.sortedStructNames()
	for _, name := range structNames {
		s := g.structs[name]
		goStructName := toGoType(name)
		if ok, elem := g.isSpanType(name); ok {
			goElem := toGoType(elem)
			fmt.Fprintf(&buf, "type %s struct {\n\tptr *%s\n\tsize uintptr\n}\n\n", goStructName, goElem)
			fmt.Fprintf(&buf, "func %sToSlice(s %s) []%s {\n", goStructName, goStructName, goElem)
			buf.WriteString("\tif s.ptr == nil { return nil }\n")
			buf.WriteString("\treturn unsafe.Slice(s.ptr, int(s.size))\n")
			buf.WriteString("}\n\n")
			fmt.Fprintf(&buf, "func sliceTo%s(s []%s) %s {\n", goStructName, goElem, goStructName)
			fmt.Fprintf(&buf, "\tif len(s) == 0 { return %s{} }\n", goStructName)
			fmt.Fprintf(&buf, "\treturn %s{\n", goStructName)
			buf.WriteString("\t\tptr: &s[0],\n")
			buf.WriteString("\t\tsize: uintptr(len(s)),\n")
			buf.WriteString("\t}\n}\n\n")
			continue
		}
		if s.IsOpaque {
			fmt.Fprintf(&buf, "type %s struct {\n\topaque unsafe.Pointer\n}\n\n", goStructName)
			if hasInit[name] && hasDrop[name] {
				fmt.Fprintf(&buf, "func New%s() *%s {\n\ts := &%s{}\n\ts.Init()\n\truntime.SetFinalizer(s, (*%s).Drop)\n\treturn s\n}\n\n", goStructName, goStructName, goStructName, goStructName)
			}
		} else {
			fmt.Fprintf(&buf, "type %s struct {\n", goStructName)
			for _, f := range s.Fields {
				typeName := g.mapCTypeStringToGo(f.Type)
				if _, isCb := g.callbacks[typeName]; isCb {
					typeName = "uintptr"
				} else {
					typeName = toGoType(typeName)
				}
				fmt.Fprintf(&buf, "\t%s %s\n", sanitizeGoNamePrivate(f.Name), typeName)
			}
			buf.WriteString("}\n\n")
			if hasInit[name] && hasDrop[name] {
				fmt.Fprintf(&buf, "func New%s() *%s {\n\ts := &%s{}\n\ts.Init()\n\truntime.SetFinalizer(s, (*%s).Drop)\n\treturn s\n}\n\n", goStructName, goStructName, goStructName, goStructName)
			}
		}
	}

	cbNames := g.sortedCallbackNames()
	for _, name := range cbNames {
		cb := g.callbacks[name]
		goCbName := toGoType(name)
		fmt.Fprintf(&buf, "type %s func(", goCbName)
		first := true
		for _, arg := range cb.Args {
			if strings.Contains(arg.Name, "userData") {
				continue
			}
			if !first {
				buf.WriteString(", ")
			}
			first = false
			goArgType := toGoType(arg.Type)
			fmt.Fprintf(&buf, "%s %s", sanitizeGoName(arg.Name), goArgType)
		}
		buf.WriteString(")")
		if cb.ReturnType != "" {
			goRetType := toGoType(cb.ReturnType)
			buf.WriteString(" ")
			buf.WriteString(goRetType)
		}
		buf.WriteString("\n\n")
	}

	for _, f := range g.functions {
		if !strings.HasPrefix(f.Name, "Discord_") {
			continue
		}
		origGoName := strings.TrimPrefix(f.Name, "Discord_")
		goName := sanitizeGoName(origGoName)
		isMethod, receiverName, receiverType := false, "", ""
		if len(f.Args) > 0 && strings.HasPrefix(f.Args[0].Type, "*Discord_") {
			isMethod, receiverType = true, toGoType(f.Args[0].Type)
			structNameStripped := stripDiscordPrefix(strings.TrimPrefix(f.Args[0].Type, "*"))
			goName = strings.TrimPrefix(goName, structNameStripped)
			receiverName = "self"
		}
		var filteredArgs []Arg
		for _, arg := range f.Args {
			if strings.Contains(arg.Name, "userData") || strings.Contains(arg.Name, "userDataFree") {
				continue
			}
			filteredArgs = append(filteredArgs, arg)
		}
		buf.WriteString("func ")
		if isMethod {
			fmt.Fprintf(&buf, "(%s %s) %s(", receiverName, receiverType, goName)
			for i := 1; i < len(filteredArgs); i++ {
				if i > 1 {
					buf.WriteString(", ")
				}
				goArgType := toGoType(filteredArgs[i].Type)
				fmt.Fprintf(&buf, "%s %s", sanitizeGoName(filteredArgs[i].Name), goArgType)
			}
		} else {
			fmt.Fprintf(&buf, "%s(", goName)
			for i, arg := range filteredArgs {
				if i > 0 {
					buf.WriteString(", ")
				}
				goArgType := toGoType(arg.Type)
				fmt.Fprintf(&buf, "%s %s", sanitizeGoName(arg.Name), goArgType)
			}
		}
		buf.WriteString(")")
		var returnTypes []string
		if f.ReturnsStruct {
			goRetType := toGoType(f.ReturnType)
			returnTypes = append(returnTypes, goRetType)
		}
		if f.OutArg != nil {
			goOutType := toGoType(f.OutArg.Type)
			returnTypes = append(returnTypes, goOutType)
		}
		if f.ReturnType != "" && !f.ReturnsStruct {
			goRetType := toGoType(f.ReturnType)
			returnTypes = append(returnTypes, goRetType)
		}
		if len(returnTypes) == 1 {
			buf.WriteString(" ")
			buf.WriteString(returnTypes[0])
		} else if len(returnTypes) > 1 {
			buf.WriteString(" (")
			buf.WriteString(strings.Join(returnTypes, ", "))
			buf.WriteString(")")
		}
		buf.WriteString(" {\n")
		if f.ReturnsStruct {
			goRetType := toGoType(f.ReturnType)
			fmt.Fprintf(&buf, "\tvar res_ret %s\n", goRetType)
		}
		if f.OutArg != nil {
			goOutType := toGoType(f.OutArg.Type)
			fmt.Fprintf(&buf, "\tvar res_out %s\n", goOutType)
		}
		var callArgs []string
		if f.ReturnsStruct && runtime.GOARCH == "amd64" {
			callArgs = append(callArgs, "uintptr(unsafe.Pointer(&res_ret))")
		}
		for _, arg := range f.Args {
			argVar := sanitizeGoName(arg.Name)
			if isMethod && arg.Name == f.Args[0].Name {
				argVar = receiverName
			}
			if strings.Contains(arg.Name, "userDataFree") {
				callArgs = append(callArgs, "0")
				continue
			}
			if strings.Contains(arg.Name, "userData") {
				var targetCb *Arg
				for _, a := range f.Args {
					if _, ok := g.callbacks[a.Type]; ok {
						targetCb = &a
						fmt.Fprintf(&buf, "\tptr_%s := registerCallback(%s)\n", arg.Name, sanitizeGoName(a.Name))
						callArgs = append(callArgs, fmt.Sprintf("uintptr(ptr_%s)", arg.Name))
						break
					}
				}
				if targetCb == nil {
					callArgs = append(callArgs, "0")
				}
				continue
			}
			targetFFIType := toGoType(g.mapToFFIType(arg.Type, arg.CType))
			if _, ok := g.callbacks[arg.Type]; ok {
				fmt.Fprintf(&buf, "\tcb_%s := purego.NewCallback(gateway_%s)\n", argVar, arg.Type)
				callArgs = append(callArgs, fmt.Sprintf("cb_%s", argVar))
				continue
			}
			if arg.Type == "string" {
				fmt.Fprintf(&buf, "\tc_%s := stringToDiscordString(%s)\n", argVar, argVar)
				callArgs = append(callArgs, fmt.Sprintf("uintptr(unsafe.Pointer(&c_%s))", argVar))
			} else if g.isKnownStruct(arg.Type) || strings.HasSuffix(arg.CType, "Span") {
				callArgs = append(callArgs, fmt.Sprintf("uintptr(unsafe.Pointer(&%s))", argVar))
			} else if strings.HasPrefix(arg.Type, "*") {
				callArgs = append(callArgs, fmt.Sprintf("uintptr(unsafe.Pointer(%s))", argVar))
			} else if arg.IsBool {
				fmt.Fprintf(&buf, "\tvar b_%s uintptr\n\tif %s { b_%s = 1 }\n", argVar, argVar, argVar)
				callArgs = append(callArgs, fmt.Sprintf("b_%s", argVar))
			} else {
				callArgs = append(callArgs, fmt.Sprintf("%s(%s)", targetFFIType, argVar))
			}
		}
		if f.OutArg != nil {
			callArgs = append(callArgs, "uintptr(unsafe.Pointer(&res_out))")
		}
		callStr := fmt.Sprintf("_%s(%s)", f.Name, strings.Join(callArgs, ", "))
		if f.ReturnType == "" || f.ReturnsStruct {
			buf.WriteString("\t")
			buf.WriteString(callStr)
			buf.WriteString("\n")
		} else {
			buf.WriteString("\tres_c := ")
			buf.WriteString(callStr)
			buf.WriteString("\n")
		}
		var retVars []string
		if f.ReturnsStruct {
			retVars = append(retVars, "res_ret")
		}
		if f.OutArg != nil {
			retVars = append(retVars, "res_out")
		}
		if f.ReturnType != "" && !f.ReturnsStruct {
			goRetType := toGoType(f.ReturnType)
			if f.ReturnType == "string" {
				retVars = append(retVars, "discordStringToString((*String)(unsafe.Pointer(res_c)))")
			} else if f.ReturnIsBool {
				retVars = append(retVars, "res_c != 0")
			} else {
				retVars = append(retVars, fmt.Sprintf("%s(res_c)", goRetType))
			}
		}
		if len(retVars) > 0 {
			buf.WriteString("\treturn ")
			buf.WriteString(strings.Join(retVars, ", "))
			buf.WriteString("\n")
		}
		buf.WriteString("}\n\n")
	}

	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		fmt.Printf("Warning: could not format source: %v\n", err)
		if err := os.WriteFile("discord.go.err", buf.Bytes(), 0644); err != nil {
			fmt.Printf("Error writing discord.go.err: %v\n", err)
		}
		return
	}
	if err := os.WriteFile("discord.go", formatted, 0644); err != nil {
		log.Fatalf("Error writing discord.go: %v", err)
	}
	fmt.Println("Generated discord.go")

	// Generate OS-specific helpers
	g.generateLibraryHelpers()
}

func (g *Generator) generateLibraryHelpers() {
	windowsBuf := []byte("// Code generated by gen; DO NOT EDIT.\n\n//go:build windows\n\npackage discord\n\nimport \"syscall\"\n\nfunc openLibraryInternal(name string) (uintptr, error) {\n\th, err := syscall.LoadLibrary(name)\n\treturn uintptr(h), err\n}\n")
	unixBuf := []byte("// Code generated by gen; DO NOT EDIT.\n\n//go:build !windows\n\npackage discord\n\nimport \"github.com/ebitengine/purego\"\n\nfunc openLibraryInternal(name string) (uintptr, error) {\n\treturn purego.Dlopen(name, purego.RTLD_NOW)\n}\n")

	if err := os.WriteFile("load_windows.go", windowsBuf, 0644); err != nil {
		log.Fatalf("Error writing load_windows.go: %v", err)
	}
	if err := os.WriteFile("load_unix.go", unixBuf, 0644); err != nil {
		log.Fatalf("Error writing load_unix.go: %v", err)
	}
}

func (g *Generator) isKnownStruct(name string) bool { _, ok := g.structs[name]; return ok }
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
