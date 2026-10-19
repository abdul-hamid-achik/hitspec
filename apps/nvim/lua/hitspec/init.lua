local M = {}

function M.setup(opts)
  opts = opts or {}

  -- ftdetect/ is only sourced for directories on runtimepath at startup; the
  -- plugin usually lives in a repository subdirectory that a plugin manager
  -- appends later, so register the filetypes here too (idempotent).
  vim.filetype.add({
    extension = {
      http = "hitspec",
      hitspec = "hitspec",
    },
  })

  -- Register snippets if luasnip is available
  local ok, _ = pcall(require, "luasnip")
  if ok then
    require("hitspec.snippets").setup()
  end
end

return M
