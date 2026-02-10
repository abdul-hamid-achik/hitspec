local M = {}

function M.setup(opts)
  opts = opts or {}

  -- Register snippets if luasnip is available
  local ok, _ = pcall(require, "luasnip")
  if ok then
    require("hitspec.snippets").setup()
  end
end

return M
