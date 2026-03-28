// Package config provides shell completion scripts.
package config

// CompletionScript returns the shell completion script for the given shell.
func CompletionScript(target string) (string, error) {
	switch target {
	case "bash":
		return bashCompletionScript(), nil
	case "zsh":
		return zshCompletionScript(), nil
	case "fish":
		return fishCompletionScript(), nil
	default:
		return "", nil
	}
}

// ValidCompletionTarget checks if the completion target is valid.
func ValidCompletionTarget(target string) bool {
	return target == "bash" || target == "zsh" || target == "fish"
}

func bashCompletionScript() string {
	return `# bash completion for gosearch
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
`
}

func zshCompletionScript() string {
	return `#compdef gosearch
_gosearch_completion() {
  _arguments \
    '-i[case-insensitive matching]' \
    '-n[show line numbers]' \
    '-w[whole-word matching]' \
    '-workers[worker pool size]:workers:' \
    '-max-size[max file size]:size:' \
    '-extensions[extensions list]:exts:' \
    '-exclude-dir[exclude dirs]:dirs:' \
    '-count[count only]' \
    '-quiet[quiet mode]' \
    '-color[color output]' \
    '-abs[absolute path output]' \
    '-format[output format]:format:(plain json)' \
    '-regex[regex mode]' \
    '-follow-symlinks[follow symlinks]' \
    '-max-depth[max traversal depth]:depth:' \
    '-dynamic-workers[dynamic scaling]' \
    '-io-workers[io workers]:count:' \
    '-cpu-workers[cpu workers]:count:' \
    '-max-workers[max cpu workers]:count:' \
    '-backpressure[channel buffer size]:count:' \
    '-metrics[print metrics]' \
    '-debug[debug logging]' \
    '-trace[verbose trace]' \
    '-monitor-goroutines[monitor goroutine count]' \
    '-monitor-interval-ms[monitor interval ms]:ms:' \
    '-cpuprofile[cpu profile file]:file:_files' \
    '-memprofile[mem profile file]:file:_files' \
    '-config[config file]:file:_files' \
    '-completion[print shell completion]:shell:(bash zsh fish)' \
    '-version[print version]' \
    '*:args:_files'
}
_gosearch_completion "$@"
`
}

func fishCompletionScript() string {
	return `complete -c gosearch -l i -d 'case-insensitive matching'
complete -c gosearch -l n -d 'show line numbers'
complete -c gosearch -l w -d 'whole-word matching'
complete -c gosearch -l workers -r -d 'worker pool size'
complete -c gosearch -l max-size -r -d 'max file size'
complete -c gosearch -l extensions -r -d 'extensions list'
complete -c gosearch -l exclude-dir -r -d 'exclude directories'
complete -c gosearch -l count -d 'count only'
complete -c gosearch -l quiet -d 'quiet mode'
complete -c gosearch -l color -d 'color output'
complete -c gosearch -l abs -d 'absolute paths'
complete -c gosearch -l format -r -a 'plain json' -d 'output format'
complete -c gosearch -l regex -d 'regex mode'
complete -c gosearch -l follow-symlinks -d 'follow symlinks'
complete -c gosearch -l max-depth -r -d 'max traversal depth'
complete -c gosearch -l dynamic-workers -d 'dynamic cpu workers'
complete -c gosearch -l io-workers -r -d 'io worker count'
complete -c gosearch -l cpu-workers -r -d 'cpu worker count'
complete -c gosearch -l max-workers -r -d 'max cpu worker count'
complete -c gosearch -l backpressure -r -d 'channel buffer size'
complete -c gosearch -l metrics -d 'print metrics'
complete -c gosearch -l debug -d 'debug logs'
complete -c gosearch -l trace -d 'verbose trace'
complete -c gosearch -l monitor-goroutines -d 'monitor goroutines'
complete -c gosearch -l monitor-interval-ms -r -d 'monitor interval ms'
complete -c gosearch -l cpuprofile -r -d 'cpu profile output'
complete -c gosearch -l memprofile -r -d 'memory profile output'
complete -c gosearch -l config -r -d 'config file'
complete -c gosearch -l completion -r -a 'bash zsh fish' -d 'print completion script'
complete -c gosearch -l version -d 'print version'
`
}
