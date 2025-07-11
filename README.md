# Clip - a basic clipboard manager for Linux

# Installation

```bash
go install github.com/almahoozi/clip@latest
```

# Features

- Copy text to the clipboard
- Paste text from the clipboard
- Remove entries from the clipboard history
- List entries in the clipboard history
- Supports multiple entries in the clipboard history
- Supports piping text to the command
- Supports pasting specific entries by index

# Usage

```bash
$ clip -h

Usage: clip [options|text]
  -d, --delete ints[=0]   Delete items from the clipboard; if n is not provided, delete the latest item, if multiple items are present delete them, negative values are interpreted as offsets from the end (default [0])
  -D, --delete-all        Delete all items from the clipboard
  -l, --list ints[=0,0]   List items in the clipboard; if no arguments are provided, list all items, if a single argument is provided [limit] it is used as a limit. If two arguments are provided [start] [end], they are used as the range to list items (default [0,0])
  -p, --paste int[=0]     Paste the nth item from the clipboard; if n is not provided, paste the last item, negative values are interpreted as offsets from the end
  -v, --version           Print version information
```

## Copy text to the clipboard

```bash
clip "Any text you want to copy"
```

or pipe text to it:

```bash
echo "Any text you want to copy" | clip
```

Adding the same text again move the entry instead of writing it, effectively
making it the latest entry.

## Paste text from the clipboard

Paste the last copied text:

```bash
clip
```

_this is equivalent to `clip -p` or `clip -p0`._

Or paste a specific entry by its index:

```bash
clip -p2
```

## Remove an entry from the clipboard history

Remove the last entry:

```bash
clip -d
```

_this is equivalent to `clip -d0`._

Or remove a specific entry by its index:

```bash
clip -d2
```

Or remove multiple entries by their indices:

```bash
clip -d2,3,5
```

## List entries in the clipboard history

```bash
clip -l
```

Or list LIMIT(5) entries:

```bash
clip -l5
```

Or list entries from Start(3) to End(5):

```bash
clip -l3,5
```

# Integrations

## Neovim

Yank:

```lua
vim.keymap.set("n", "<leader>y", function()
  local text = table.concat(vim.fn.getreg('"', 1, true), "\n")
  vim.fn.system({ "clip" }, text)
end)
```

Paste:

```lua
vim.keymap.set("n", "<leader>p", function()
  local text = vim.fn.systemlist("clip")
  vim.fn.setreg('"', text)
  vim.api.nvim_paste(table.concat(text, "\n"), true, -1)
end)
```

## Tmux

```bash
bind-key -T copy-mode-vi y send-keys -X copy-pipe-and-cancel "clip"
bind-key P run-shell "clip"
```

## FZF

Just pipe the output of `clip -l` to FZF and search interactively:

```bash
clip -l | fzf
```

_NOTE: In the future, long entries will be truncated. We will provide an option
to include the index in the list output, so that you can pipe the fzf output to
`clip -p` to paste the selected entry._

# Known Issues

- The `clip -l` command does not currently support limiting or specifying a
  range of entries to display. This feature is planned for future updates. It
  currently only lists all entries in the clipboard history.
- We store the clipboard history in a file located at
  `$XDG_DATA_HOME/clip/data.json` or `~/.local/share/clip/data.json` if
  `$XDG_DATA_HOME` is not set. We do not lock this file, so race conditions
  may occur if multiple instances of `clip` are running simultaneously.
- The file path is hardcoded.

# Future Plans

- Configure a maximum number of entries in the clipboard history.
- Implementing a TTL for entries, so they get removed after a certain time.
- Implementing a size limit for the clipboard history, removing the oldest
  entries when the limit is reached.
- Optionally truncate long items when listing entries.
- Implementing a memory-only mode, where entries are not persisted to disk.
- Implementing a memory caching agent so that disk syncing is done in the
  background, improving performance.
- Improve clipboard management, allowing moving entries around.
- Allow formatting of list output with created date, TTL, last used date, etc.
- Implement "frecency" scoring to sort entries based on usage frequency and recency.
- Improve persisted data format for better performance and flexibility.
- Support encrypted clipboard history/items.
- Allow storing a separate clipboard history for each user, or using a shared
  clipboard history across users.
- Allow named entries, so they can be referenced by name instead of index.
  Those would persist forever, unless the user manually removes them.
- Allow manually setting the expiration date for entries.
- Allow configuring the clipboard history file path.
