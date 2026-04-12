---
title: How to prevent a MacBook from sleeping
description: Keep a MacBook awake as a runner host using pmset plus caffeinate.
---

`pmset` alone is **not** enough on a MacBook — the system still sleeps on lid-close or idle user-session timeouts. You need `pmset` (persistent settings) **and** a long-running `caffeinate -simdu` (active sleep assertion).

## TL;DR

```sh
sudo pmset -a sleep 0 disksleep 0 hibernatemode 0 standby 0 autopoweroff 0 powernap 0
sudo pmset -a disablesleep 1

brew install tmux
tmux new -d -s caffeinate 'caffeinate -simdu'
```

Read on for what each piece does and alternatives to `tmux`.

## 1. Persistent settings with `pmset`

Plug in AC power, then:

```sh
sudo pmset -a sleep 0 disksleep 0 hibernatemode 0 standby 0 autopoweroff 0 powernap 0
sudo pmset -a disablesleep 1    # allow lid-closed operation without an external display
```

Verify:

```sh
pmset -g | grep -E 'sleep|hibernatemode|standby'
```

## 2. Active assertion with `caffeinate`

Keep a `caffeinate` process running for as long as you want the Mac awake:

```sh
caffeinate -simdu
```

Flags:

| Flag | Prevents                     |
|------|------------------------------|
| `-s` | system sleep (AC)            |
| `-i` | idle sleep                   |
| `-m` | disk sleep                   |
| `-d` | display sleep                |
| `-u` | declares user activity       |

Pick whichever way keeps it running long enough for your use case.

### Option A: Dedicated terminal window

Open a new Terminal/iTerm/Ghostty window and run:

```sh
caffeinate -simdu
```

Leave the window open. Closing the window or pressing `Ctrl-C` stops it.

### Option B: `nohup` (background, survives logout)

```sh
nohup caffeinate -simdu >/dev/null 2>&1 &
disown
```

Check it is running:

```sh
pgrep -fl caffeinate
```

Stop it:

```sh
pkill caffeinate
```

### Option C: `tmux`

```sh
brew install tmux           # if not installed
tmux new -d -s caffeinate 'caffeinate -simdu'
```

Reattach to inspect:

```sh
tmux attach -t caffeinate
```

Stop it:

```sh
tmux kill-session -t caffeinate
```

### Option D: `screen`

```sh
screen -dmS caffeinate caffeinate -simdu
```

Reattach:

```sh
screen -r caffeinate
```

Stop it:

```sh
screen -S caffeinate -X quit
```


## 3. Verify

```sh
pmset -g assertions
```

You should see `PreventUserIdleSystemSleep` and `PreventSystemSleep` held by `caffeinate`.

## Revert

```sh
pkill caffeinate
sudo pmset -a restoredefaults
```

## Troubleshooting

- **Still sleeps on lid close**: confirm `disablesleep` is `1` in `pmset -g`.
- **Docker hangs after wake**: `disksleep` must be `0`; restart Docker Desktop.
- **Something else forces sleep**: check `pmset -g assertions` for other processes (backup tools, VPNs) holding conflicting assertions.
