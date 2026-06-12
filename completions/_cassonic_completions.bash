# bash completion for cassonic
# Source this file or add to ~/.bashrc:
#   eval "$(cassonic --shell init bash)"
_cassonic() {
    local cur prev
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"

    case "$prev" in
        --mode)
            COMPREPLY=($(compgen -W "production development" -- "$cur"))
            return
            ;;
        --config|--data|--cache|--log|--backup|--backup-dir|--pid|--tor-key|--tls-cert|--tls-key)
            COMPREPLY=($(compgen -d -- "$cur"))
            return
            ;;
        --service)
            COMPREPLY=($(compgen -W "start restart stop reload --install --uninstall --disable --help" -- "$cur"))
            return
            ;;
        --maintenance)
            COMPREPLY=($(compgen -W "backup restore update mode setup --help" -- "$cur"))
            return
            ;;
        --update)
            COMPREPLY=($(compgen -W "check yes branch=stable branch=beta branch=daily" -- "$cur"))
            return
            ;;
        --lang)
            COMPREPLY=($(compgen -W "en es fr de zh ja ar" -- "$cur"))
            return
            ;;
        --color)
            COMPREPLY=($(compgen -W "auto yes no" -- "$cur"))
            return
            ;;
        --shell)
            COMPREPLY=($(compgen -W "completions init --help" -- "$cur"))
            return
            ;;
        --address|--port|--baseurl|--tls-domain|--tls-email)
            return
            ;;
    esac

    local flags="--help --version --mode --config --data --cache --log --address --port --baseurl --debug --scan --status --pid --install --uninstall --backup --tor-key --service --daemon --maintenance --update --lang --color --shell --tls --tls-domain --tls-email --tls-cert --tls-key"
    COMPREPLY=($(compgen -W "$flags" -- "$cur"))
}
complete -F _cassonic cassonic
