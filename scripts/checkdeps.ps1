$ErrorActionPreference = 'Stop'
$fail = 0
function Check([string]$pkg, [string[]]$deps) {
  $script:fail += 0
  $listed = & go list -deps $pkg 2>$null
  foreach ($dep in $deps) {
    if ($listed -contains $dep) { Write-Output "VIOLATION: $pkg depends on $dep"; $script:fail = 1 }
  }
}
Check './internal/core'  @('github.com/MC5lan/HonestBoy-CLI/internal/tui','github.com/MC5lan/HonestBoy-CLI/internal/llm','github.com/MC5lan/HonestBoy-CLI/internal/tools','github.com/MC5lan/HonestBoy-CLI/internal/store','github.com/MC5lan/HonestBoy-CLI/internal/engine')
Check './internal/llm'   @('github.com/MC5lan/HonestBoy-CLI/internal/tui','github.com/MC5lan/HonestBoy-CLI/internal/cli')
Check './internal/tools' @('github.com/MC5lan/HonestBoy-CLI/internal/tui','github.com/MC5lan/HonestBoy-CLI/internal/cli')
Check './internal/tui'   @('github.com/MC5lan/HonestBoy-CLI/internal/llm','github.com/MC5lan/HonestBoy-CLI/internal/tools','github.com/MC5lan/HonestBoy-CLI/internal/store')
Check './internal/cli'   @('github.com/MC5lan/HonestBoy-CLI/internal/tui')
exit $fail