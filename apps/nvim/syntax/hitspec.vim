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
syn match hitspecComment /^#$/ contains=hitspecTodo
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
syn match hitspecAnnotation /^#\s*@waitfor\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@import\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@stress\.\w\+\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@stress\.\w\+\s*$/ contains=hitspecAnnotationKey
syn match hitspecAnnotation /^#\s*@contract\.\w\+\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue
syn match hitspecAnnotation /^#\s*@x-[a-zA-Z0-9._-]\+\s.*$/ contains=hitspecAnnotationKey,hitspecAnnotationValue

syn match hitspecAnnotationKey /@[a-zA-Z0-9._-]\+/ contained
syn match hitspecAnnotationValue /\s\zs[^@#].*$/ contained

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
syn match hitspecVarEquals /=/ contained

" Variable interpolation: {{...}}
syn region hitspecVariable matchgroup=hitspecVariableBrace start=/{{/ end=/}}/ oneline contains=hitspecBuiltinFunc

" Built-in functions: $func()
syn match hitspecBuiltinFunc /\$\w\+([^)]*)/ contained contains=hitspecFuncName
syn match hitspecFuncName /\$\w\+/ contained

" Assertion block markers
syn match hitspecAssertStart /^>>>\w*$/
syn match hitspecAssertStart /^>>>$/
syn match hitspecAssertEnd /^<<<$/

" Typed block markers
syn match hitspecBlockType /^>>>capture$/
syn match hitspecBlockType /^>>>graphql$/
syn match hitspecBlockType /^>>>db$/
syn match hitspecBlockType /^>>>shell$/
syn match hitspecBlockType /^>>>multipart$/
syn match hitspecBlockType /^>>>variables$/

" Legacy capture blocks
syn match hitspecCaptureStart /^\[\[\[$/
syn match hitspecCaptureEnd /^\]\]\]$/

" Assertion keywords
syn keyword hitspecExpect expect
syn keyword hitspecAssertSubject status body header duration headers contained containedin=hitspecExpect

" Assertion operators
syn keyword hitspecOperator == != contains matches exists startsWith endsWith includes type schema snapshot each in length
syn match hitspecOperator /!exists/
syn match hitspecOperator /!contains/
syn match hitspecOperator /!includes/
syn match hitspecOperator /!in/
syn match hitspecOperator /[><]=\?/

" Capture keyword
syn keyword hitspecFrom from

" Strings
syn region hitspecString start=/"/ skip=/\\"/ end=/"/ oneline contains=hitspecVariable
syn region hitspecString start=/'/ skip=/\\'/ end=/'/ oneline

" Numbers
syn match hitspecNumber /\<\d\+\>/
syn match hitspecNumber /\<\d\+\.\d\+\>/

" Booleans and null
syn keyword hitspecBoolean true false
syn keyword hitspecNull null

" URL (after method)
syn match hitspecURL /\(GET\|POST\|PUT\|PATCH\|DELETE\|HEAD\|OPTIONS\|TRACE\|CONNECT\|WS\)\s\+\zs\S\+/ contains=hitspecVariable

" Query parameters
syn match hitspecQueryParam /^\s\+[?&]\w[^=]*=.*$/ contains=hitspecVariable

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
hi def link hitspecExpect Keyword
hi def link hitspecAssertSubject Type
hi def link hitspecOperator Operator
hi def link hitspecFrom Keyword
hi def link hitspecString String
hi def link hitspecNumber Number
hi def link hitspecBoolean Boolean
hi def link hitspecNull Constant
hi def link hitspecURL Underlined
hi def link hitspecQueryParam Normal
hi def link hitspecJsonKey Identifier

let b:current_syntax = "hitspec"
