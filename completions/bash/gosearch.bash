# bash completion for gosearch
_gosearch_completion() {
  local cur prev
  COMPREPLY=()
  cur="${COMP_WORDS[COMP_CWORD]}"
  prev="${COMP_WORDS[COMP_CWORD-1]}"
  local opts="-i -n -w -workers -max-size -extensions -exclude-dir -count -quiet -color -abs -format -regex -follow-symlinks -max-depth -dynamic-workers -io-workers -cpu-workers -max-workers -backpressure -metrics -debug -trace -monitor-goroutines -monitor-interval-ms -cpuprofile -memprofile -config -completion -version"
  case "$prev" in
    -format)
      COMPREPLY=( $(compgen -W "plain json" -- "$cur") )
      return 0
      ;;
    -completion)
      COMPREPLY=( $(compgen -W "bash zsh fish" -- "$cur") )
      return 0
      ;;
  esac
  if [[ "$cur" == -* ]]; then
    COMPREPLY=( $(compgen -W "$opts" -- "$cur") )
  fi
}
complete -F _gosearch_completion gosearch
