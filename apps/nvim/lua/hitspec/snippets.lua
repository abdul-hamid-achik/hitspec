local M = {}

function M.setup()
  local ok, ls = pcall(require, "luasnip")
  if not ok then
    return
  end

  local s = ls.snippet
  local t = ls.text_node
  local i = ls.insert_node
  local c = ls.choice_node

  local snippets = {
    -- HTTP method templates
    s("get", {
      t("### "), i(1, "Request Name"),
      t({ "", "GET " }), i(2, "{{baseUrl}}/endpoint"),
      t({ "", "" }),
      i(0),
    }),

    s("post", {
      t("### "), i(1, "Request Name"),
      t({ "", "POST " }), i(2, "{{baseUrl}}/endpoint"),
      t({ "", "Content-Type: application/json", "", "" }),
      t("{"), t({ "", '  "' }), i(3, "key"), t('": "'), i(4, "value"), t('"'),
      t({ "", "}" }),
      t({ "", "" }),
      i(0),
    }),

    s("put", {
      t("### "), i(1, "Request Name"),
      t({ "", "PUT " }), i(2, "{{baseUrl}}/endpoint"),
      t({ "", "Content-Type: application/json", "", "" }),
      t("{"), t({ "", '  "' }), i(3, "key"), t('": "'), i(4, "value"), t('"'),
      t({ "", "}" }),
      t({ "", "" }),
      i(0),
    }),

    s("patch", {
      t("### "), i(1, "Request Name"),
      t({ "", "PATCH " }), i(2, "{{baseUrl}}/endpoint"),
      t({ "", "Content-Type: application/json", "", "" }),
      t("{"), t({ "", '  "' }), i(3, "key"), t('": "'), i(4, "value"), t('"'),
      t({ "", "}" }),
      t({ "", "" }),
      i(0),
    }),

    s("delete", {
      t("### "), i(1, "Request Name"),
      t({ "", "DELETE " }), i(2, "{{baseUrl}}/endpoint"),
      t({ "", "" }),
      i(0),
    }),

    s("head", {
      t("### "), i(1, "Request Name"),
      t({ "", "HEAD " }), i(2, "{{baseUrl}}/endpoint"),
      t({ "", "" }),
      i(0),
    }),

    s("options", {
      t("### "), i(1, "Request Name"),
      t({ "", "OPTIONS " }), i(2, "{{baseUrl}}/endpoint"),
      t({ "", "" }),
      i(0),
    }),

    s("ws", {
      t("### "), i(1, "WebSocket"),
      t({ "", "WS " }), i(2, "{{baseUrl}}/ws"),
      t({ "", "" }),
      i(0),
    }),

    -- Assertion block
    s("assert", {
      t({ ">>>", "" }),
      t("expect "), i(1, "status"), t(" "), i(2, "=="), t(" "), i(3, "200"),
      t({ "", "<<<" }),
      t({ "", "" }),
      i(0),
    }),

    -- Capture block
    s("capture", {
      t({ ">>>capture", "" }),
      i(1, "varName"), t(" from "), i(2, "body"), t(" "), i(3, "$.path"),
      t({ "", "<<<" }),
      t({ "", "" }),
      i(0),
    }),

    -- GraphQL block
    s("graphql", {
      t("### "), i(1, "GraphQL Query"),
      t({ "", "POST " }), i(2, "{{baseUrl}}/graphql"),
      t({ "", "Content-Type: application/json", "", "" }),
      t({ ">>>graphql", "" }),
      t("query {"),
      t({ "", "  " }), i(3, "field"), t(" {"),
      t({ "", "    " }), i(4, "id"),
      t({ "", "  }" }),
      t({ "", "}" }),
      t({ "", "<<<" }),
      t({ "", "" }),
      i(0),
    }),

    -- DB block
    s("db", {
      t({ ">>>db", "" }),
      t("SELECT "), i(1, "*"), t(" FROM "), i(2, "table"), t(" WHERE "), i(3, "id = 1"),
      t({ "", "<<<" }),
      t({ "", "" }),
      i(0),
    }),

    -- Shell block
    s("shell", {
      t({ ">>>shell", "" }),
      i(1, "echo 'hello'"),
      t({ "", "<<<" }),
      t({ "", "" }),
      i(0),
    }),

    -- Multipart block
    s("multipart", {
      t({ ">>>multipart", "" }),
      i(1, "field"), t(" = "), i(2, "value"),
      t({ "", "<<<" }),
      t({ "", "" }),
      i(0),
    }),

    -- Variables block
    s("variables", {
      t({ ">>>variables", "" }),
      t("@"), i(1, "varName"), t(" = "), i(2, "value"),
      t({ "", "<<<" }),
      t({ "", "" }),
      i(0),
    }),

    -- Annotations
    s("@name", { t("# @name "), i(1, "requestName"), t({ "", "" }), i(0) }),
    s("@description", { t("# @description "), i(1, "Description"), t({ "", "" }), i(0) }),
    s("@tags", { t("# @tags "), i(1, "tag1, tag2"), t({ "", "" }), i(0) }),
    s("@timeout", { t("# @timeout "), i(1, "5000"), t({ "", "" }), i(0) }),
    s("@retry", { t("# @retry "), i(1, "3"), t({ "", "" }), i(0) }),
    s("@retryOn", { t("# @retryOn "), i(1, "500, 502, 503"), t({ "", "" }), i(0) }),
    s("@retryDelay", { t("# @retryDelay "), i(1, "1000"), t({ "", "" }), i(0) }),
    s("@if", { t("# @if "), i(1, "{{condition}}"), t({ "", "" }), i(0) }),
    s("@unless", { t("# @unless "), i(1, "{{condition}}"), t({ "", "" }), i(0) }),
    s("@depends", { t("# @depends "), i(1, "requestName"), t({ "", "" }), i(0) }),
    s("@auth", {
      t("# @auth "),
      c(2, {
        t("bearer {{token}}"),
        t("basic {{user}} {{pass}}"),
        t("apikey X-API-Key {{key}}"),
        t("digest {{user}} {{pass}}"),
        t("aws {{region}} {{service}}"),
      }),
      t({ "", "" }),
      i(0),
    }),
    s("@skip", { t("# @skip"), t({ "", "" }), i(0) }),
    s("@only", { t("# @only"), t({ "", "" }), i(0) }),
    s("@before", { t("# @before "), i(1, "command"), t({ "", "" }), i(0) }),
    s("@after", { t("# @after "), i(1, "command"), t({ "", "" }), i(0) }),
    s("@db", { t("# @db "), i(1, "postgres://user:pass@host/db"), t({ "", "" }), i(0) }),
    s("@waitfor", { t("# @waitfor "), i(1, "http://localhost:8080/health"), t(" "), i(2, "200"), t(" "), i(3, "30000"), t({ "", "" }), i(0) }),
    s("@import", { t("# @import "), i(1, "./shared.http"), t({ "", "" }), i(0) }),

    -- Stress annotations
    s("@stress.weight", { t("# @stress.weight "), i(1, "1"), t({ "", "" }), i(0) }),
    s("@stress.think", { t("# @stress.think "), i(1, "500"), t({ "", "" }), i(0) }),
    s("@stress.skip", { t("# @stress.skip"), t({ "", "" }), i(0) }),
    s("@stress.setup", { t("# @stress.setup"), t({ "", "" }), i(0) }),
    s("@stress.teardown", { t("# @stress.teardown"), t({ "", "" }), i(0) }),

    -- Contract annotations
    s("@contract.state", { t("# @contract.state "), i(1, "state description"), t({ "", "" }), i(0) }),
    s("@contract.provider", { t("# @contract.provider "), i(1, "provider-name"), t({ "", "" }), i(0) }),

    -- Built-in functions
    s("$uuid", { t("{{$uuid()}}"), i(0) }),
    s("$timestamp", { t("{{$timestamp()}}"), i(0) }),
    s("$timestampMs", { t("{{$timestampMs()}}"), i(0) }),
    s("$now", { t("{{$now("), i(1, "2006-01-02"), t(")}}"), i(0) }),
    s("$random", { t("{{$random("), i(1, "0"), t(", "), i(2, "1000"), t(")}}"), i(0) }),
    s("$randomString", { t("{{$randomString("), i(1, "16"), t(")}}"), i(0) }),
    s("$randomEmail", { t("{{$randomEmail()}}"), i(0) }),
    s("$randomAlphanumeric", { t("{{$randomAlphanumeric("), i(1, "8"), t(")}}"), i(0) }),
    s("$base64", { t("{{$base64("), i(1, "text"), t(")}}"), i(0) }),
    s("$base64Decode", { t("{{$base64Decode("), i(1, "encoded"), t(")}}"), i(0) }),
    s("$md5", { t("{{$md5("), i(1, "text"), t(")}}"), i(0) }),
    s("$sha256", { t("{{$sha256("), i(1, "text"), t(")}}"), i(0) }),
    s("$urlEncode", { t("{{$urlEncode("), i(1, "text"), t(")}}"), i(0) }),
    s("$urlDecode", { t("{{$urlDecode("), i(1, "text"), t(")}}"), i(0) }),
    s("$date", { t("{{$date("), i(1, "2006-01-02"), t(")}}"), i(0) }),
    s("$env", { t("{{$env("), i(1, "VAR_NAME"), t(")}}"), i(0) }),

    -- Complete request template
    s("request", {
      t("### "), i(1, "Request Name"),
      t({ "", "# @name " }), i(2, "requestName"),
      t({ "", "# @tags " }), i(3, "api"),
      t({ "", "" }),
      c(4, {
        t("GET"),
        t("POST"),
        t("PUT"),
        t("PATCH"),
        t("DELETE"),
      }),
      t(" "), i(5, "{{baseUrl}}/endpoint"),
      t({ "", "Content-Type: application/json" }),
      t({ "", "Authorization: Bearer {{token}}" }),
      t({ "", "", "{" }),
      t({ "", '  "' }), i(6, "key"), t('": "'), i(7, "value"), t('"'),
      t({ "", "}" }),
      t({ "", "", ">>>", "" }),
      t("expect status == "), i(8, "200"),
      t({ "", "expect body." }), i(9, "id"), t(" exists"),
      t({ "", "<<<" }),
      t({ "", "", ">>>capture", "" }),
      i(10, "id"), t(" from body $."), i(11, "id"),
      t({ "", "<<<" }),
      t({ "", "" }),
      i(0),
    }),
  }

  ls.add_snippets("hitspec", snippets)
end

return M
