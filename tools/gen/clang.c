#include <clang-c/Index.h>

enum CXChildVisitResult goVisitCursor(CXCursor *cursor, CXCursor *parent);
enum CXChildVisitResult goVisitCursorEnum(CXCursor *cursor, CXCursor *parent, CXClientData client_data);
enum CXChildVisitResult goVisitCursorStruct(CXCursor *cursor, CXCursor *parent, CXClientData client_data);
enum CXChildVisitResult goVisitCallbackParams(CXCursor *cursor, CXCursor *parent, CXClientData client_data);

enum CXChildVisitResult visitCursor(CXCursor cursor, CXCursor parent, CXClientData client_data) {
    return goVisitCursor(&cursor, &parent);
}

enum CXChildVisitResult visitCursorEnum(CXCursor cursor, CXCursor parent, CXClientData client_data) {
    return goVisitCursorEnum(&cursor, &parent, client_data);
}

enum CXChildVisitResult visitCursorStruct(CXCursor cursor, CXCursor parent, CXClientData client_data) {
    return goVisitCursorStruct(&cursor, &parent, client_data);
}

enum CXChildVisitResult visitCallbackParams(CXCursor cursor, CXCursor parent, CXClientData client_data) {
    return goVisitCallbackParams(&cursor, &parent, client_data);
}
