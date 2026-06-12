#compdef cassonic
# zsh completion for cassonic
# Source this file or add to ~/.zshrc:
#   eval "$(cassonic --shell init zsh)"
_cassonic() {
    _arguments \
        '(--help -h)'{--help,-h}'[Show help]' \
        '(--version -v)'{--version,-v}'[Show version]' \
        '--mode[Server mode]:mode:(production development)' \
        '--config[Config directory]:dir:_files -/' \
        '--data[Data directory]:dir:_files -/' \
        '--cache[Cache directory]:dir:_files -/' \
        '--log[Log directory]:dir:_files -/' \
        '--address[Listen address]:address:' \
        '--port[Listen port]:port:' \
        '--baseurl[Base URL path prefix]:path:' \
        '--debug[Enable debug output]' \
        '--scan[Run library scan and exit]' \
        '--status[Show server status and exit]' \
        '--pid[Write PID to file]:file:_files' \
        '--install[Install as system service and exit]' \
        '--uninstall[Remove system service and exit]' \
        '--backup[Backup directory]:dir:_files -/' \
        '--tor-key[Tor ed25519 key file]:file:_files' \
        '--service[Service management command]:cmd:(start restart stop reload --install --uninstall --disable --help)' \
        '--daemon[Fork to background]' \
        '--maintenance[Maintenance operation]:cmd:(backup restore update mode setup --help)' \
        '--update[Update command]:cmd:(check yes branch=stable branch=beta branch=daily)' \
        '--lang[UI language]:lang:(en es fr de zh ja ar)' \
        '--color[Color output]:mode:(auto yes no)' \
        '--shell[Shell integration]:cmd:(completions init --help)' \
        '--tls[Enable TLS]' \
        '--tls-domain[TLS domain]:domain:' \
        '--tls-email[TLS email]:email:' \
        '--tls-cert[TLS certificate file]:file:_files' \
        '--tls-key[TLS key file]:file:_files'
}
_cassonic
