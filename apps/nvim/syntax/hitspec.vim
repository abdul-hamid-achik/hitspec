" Vim syntax file
" Language: hitspec (.http / .hitspec)
" Maintainer: abdul-hamid-achik

if exists("b:current_syntax")
  finish
endif

" Request separators: ###
syn match hitspecSeparator /^###\s*.*$/ contains=hitspecSeparatorName
syn match hitspecSeparatorName /[^ #].*$/ contained

" Comments
syn match hitspecComment /^#\s[^@].*$/ contains=hitspecTodo
syn match hitspecComment /^#\s*$/ contains=hitspecTodo
syn match hitspecComment /^\/\/.*$/ contains=hitspecTodo
syn keyword hitspecTodo TODO FIXME XXX NOTE HACK BUG WARN contained

" Annotations
syn match hitspecAnnotation /^#\s*@name\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@description\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@tags\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@timeout\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@retry\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@retryOn\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@retryDelay\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@if\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@unless\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@depends\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@auth\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@skip\>.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@only\>.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@before\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@after\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@db\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@wait[Ff]or\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@import\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@stress\.\w\+\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@stress\.\w\+\s*$/ contains=hitspecAnnotationKey
syn match hitspecAnnotation /^#\s*@contract\.\w\+\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@x-[a-zA-Z0-9._-]\+\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue

syn match hitspecAnnotationKey /@[a-zA-Z0-9._-]\+/ contained
syn match hitspecAnnotationValue /\s\zs[^@#].*$/ contained contains=hitspecVariable

" HTTP methods
syn match hitspecMethod /^\(GET\|POST\|PUT\|PATCH\|DELETE\|HEAD\|OPTIONS\|TRACE\|CONNECT\|WS\)\ze\s/

" Headers: Key: Value
syn match hitspecHeader /^[A-Za-z][A-Za-z0-9_-]*\s*:\s*.*$/ contains=hitspecHeaderKey,hitspecHeaderColon,hitspecHeaderValue,hitspecVariable
syn match hitspecHeaderKey /^[A-Za-z][A-Za-z0-9_-]*/ contained
syn match hitspecHeaderColon /:/ contained
syn match hitspecHeaderValue /:\s*\zs.*$/ contained contains=hitspecVariable

" Variable assignment: @var = value
syn match hitspecVarAssign /^@\w\+\s*=\s*.*$/ contains=hitspecVarName,hitspecVarEquals,hitspecVariable
syn match hitspecVarName /^@\w\+/ contained
syn match hitspecVarEquals /\s\zs=\ze\s/ contained

" Variable interpolation: {{...}}
syn region hitspecVariable matchgroup=hitspecVariableBrace start=/{{/ end=/}}/ oneline contains=hitspecBuiltinFunc

" Built-in functions: $func()
syn match hitspecBuiltinFunc /\$\w\+([^)]*)/ contained contains=hitspecFuncName
syn match hitspecFuncName /\$\w\+/ contained

" Assertion block markers
syn match hitspecAssertStart /^>>>\w*$/
syn match hitspecAssertEnd /^<<<$/

" Typed block markers
syn match hitspecBlockType /^>>>capture$/
syn match hitspecBlockType /^>>>graphql$/
syn match hitspecBlockType /^>>>db$/
syn match hitspecBlockType /^>>>shell$/
syn match hitspecBlockType /^>>>mock$/
syn match hitspecBlockType /^>>>multipart$/
syn match hitspecBlockType /^>>>variables$/

" Legacy capture blocks
syn match hitspecCaptureStart /^\[\[\[$/
syn match hitspecCaptureEnd /^\]\]\]$/

" Assertion lines: expect subject operator [value]
syn match hitspecExpect /^expect\s\+.*$/ contains=hitspecExpectKeyword,hitspecAssertSubject,hitspecAssertOp,hitspecString,hitspecNumber,hitspecBoolean,hitspecNull,hitspecVariable,hitspecType
syn match hitspecExpectKeyword /^expect/ contained
syn match hitspecAssertSubject /\<\(status\|body\|header\|duration\|headers\)\>/ contained

" Assertion operators (contained — only match inside expect lines)
syn match hitspecAssertOp /\<\(contains\|matches\|exists\|startsWith\|endsWith\|includes\|type\|schema\|snapshot\|each\|in\|length\)\>/ contained
syn match hitspecAssertOp /==/ contained
syn match hitspecAssertOp /!=/ contained
syn match hitspecAssertOp /[><]=\?/ contained
syn match hitspecAssertOp /!exists/ contained
syn match hitspecAssertOp /!contains/ contained
syn match hitspecAssertOp /!includes/ contained
syn match hitspecAssertOp /!in/ contained

" Type keywords in assertions (expect body type string)
syn keyword hitspecType string number boolean array object contained

" Capture keyword
syn keyword hitspecFrom from

" File include: < ./path/to/file
syn match hitspecFileInclude /^<\s\+\S\+$/ contains=hitspecFileIncludeOp,hitspecFileIncludePath
syn match hitspecFileIncludeOp /^</ contained
syn match hitspecFileIncludePath /\s\zs\S\+$/ contained

" Multipart block keywords
syn keyword hitspecMultipartKw field file

" Database block keyword
syn keyword hitspecDBKeyword query

" Strings
syn region hitspecString start=/"/ skip=/\\"/ end=/"/ oneline contains=hitspecVariable
syn region hitspecString start=/'/ skip=/\\'/ end=/'/ oneline

" Numbers (including negative)
syn match hitspecNumber /-\?\<\d\+\>/
syn match hitspecNumber /-\?\<\d\+\.\d\+\>/

" Booleans and null
syn keyword hitspecBoolean true false
syn keyword hitspecNull null

" URL (after method)
syn match hitspecURL /\(GET\|POST\|PUT\|PATCH\|DELETE\|HEAD\|OPTIONS\|TRACE\|CONNECT\|WS\)\s\+\zs\S\+/ contains=hitspecVariable

" Query parameters and form data
syn match hitspecQueryParam /^\s*[?&]\s*\w[^=]*=.*$/ contains=hitspecVariable

" JSON body highlighting (basic)
syn match hitspecJsonKey /"[^"]*"\s*:/ contains=hitspecString

" Highlight links
hi def link hitspecSeparator Title
hi def link hitspecSeparatorName Identifier
hi def link hitspecComment Comment
hi def link hitspecTodo Todo
hi def link hitspecAnnotation PreProc
hi def link hitspecAnnotationKey Keyword
hi def link hitspecAnnotationValue String
hi def link hitspecMethod Keyword
hi def link hitspecHeader Normal
hi def link hitspecHeaderKey Type
hi def link hitspecHeaderColon Delimiter
hi def link hitspecHeaderValue String
hi def link hitspecVarAssign Normal
hi def link hitspecVarName Identifier
hi def link hitspecVarEquals Operator
hi def link hitspecVariable Special
hi def link hitspecVariableBrace Special
hi def link hitspecBuiltinFunc Function
hi def link hitspecFuncName Function
hi def link hitspecAssertStart Structure
hi def link hitspecAssertEnd Structure
hi def link hitspecBlockType Type
hi def link hitspecCaptureStart Structure
hi def link hitspecCaptureEnd Structure
hi def link hitspecExpect Normal
hi def link hitspecExpectKeyword Keyword
hi def link hitspecAssertSubject Type
hi def link hitspecAssertOp Operator
hi def link hitspecType Type
hi def link hitspecFrom Keyword
hi def link hitspecFileInclude Normal
hi def link hitspecFileIncludeOp Operator
hi def link hitspecFileIncludePath String
hi def link hitspecMultipartKw Keyword
hi def link hitspecDBKeyword Keyword
hi def link hitspecString String
hi def link hitspecNumber Number
hi def link hitspecBoolean Boolean
hi def link hitspecNull Constant
hi def link hitspecURL Underlined
hi def link hitspecQueryParam Normal
hi def link hitspecJsonKey Identifier

let b:current_syntax = "hitspec"
