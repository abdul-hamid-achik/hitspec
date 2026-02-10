-- Buffer-local settings for hitspec files

vim.bo.commentstring = "# %s"
vim.bo.shiftwidth = 2
vim.bo.tabstop = 2
vim.bo.expandtab = true
vim.bo.softtabstop = 2

-- Clean undo of ftplugin settings
vim.b.undo_ftplugin = table.concat({
  "setlocal commentstring<",
  "setlocal shiftwidth<",
  "setlocal tabstop<",
  "setlocal expandtab<",
  "setlocal softtabstop<",
}, " | ")
