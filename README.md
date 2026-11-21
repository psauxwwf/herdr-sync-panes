# Herdr Sync Panes

A Herdr plugin written in Go for broadcasting terminal input to every other
pane in the current tab. Herdr 0.9.5 does not expose a hook for keys typed in
an existing pane, so input is typed into a **separate control pane**.

## Requirements

- Herdr 0.9.5 or newer, on Linux or macOS.
- Go 1.23 or newer to build.

## Build and link locally

```sh
go build -o bin/herdr-sync-panes ./cmd/herdr-sync-panes
herdr plugin link "$PWD"
```

`herdr plugin link` does not run the manifest's build step; rebuild after
changing the Go code. Linking changes the current Herdr installation.

Add a binding of your choice to `~/.config/herdr/config.toml`:

```toml
[[keys.command]]
key = "prefix+y"
type = "plugin_action"
command = "herdr-sync-panes.start"
description = "sync input to panes"
```

Replace `prefix+y` if it is already in use, then reload:

```sh
herdr server reload-config
```

Alternatively, start it without a binding:

```sh
herdr plugin action invoke herdr-sync-panes.start
```

The action opens a split control pane in the focused pane's tab. Everything
typed into it is sent to all other panes in that tab; nothing is echoed in the
control pane. **Ctrl+]** stops synchronization and closes the control pane.
Herdr's own keybindings may intercept keys before the control pane receives
them. Target programs can interpret the same keys differently.

The target set is fixed when sync starts. If a pane is added, removed, or moved,
sync stops before sending the next input. Only one sync session per tab is
allowed. If delivery fails midway, some panes may already have received the
input; the control pane displays an error. Commands and passwords are sent to
every target: use with care.

## Development

```sh
task check
task test
task build
```

`task build-linux` produces a static Linux amd64 binary. `task release
VERSION=<tag>` packages that binary and publishes a GitHub release.

To unregister a linked plugin, use `herdr plugin unlink herdr-sync-panes`.
