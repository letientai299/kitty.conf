local root = ...
local log = hs.logger.new("kitty-qt", "warning")

return hs.hotkey.bind({ "cmd" }, "escape", function()
  local task = hs.task.new(root .. "/bin/kt", function(code, _, stderr)
    if code ~= 0 then log.e(stderr ~= "" and stderr or "quick terminal exited") end
  end, { "qt" })
  if not task or not task:start() then log.e("quick terminal start failed") end
end)
