# Changelog

## 0.1.2 (unreleased)

- Hotspot fallback: honest join errors, Instant Hotspot–friendly schedule, CoreWLAN join via menu bar app
- `keepgoing wifi test [--join]` scans for the configured hotspot without disconnecting Wi-Fi
- Config writes use read-modify-write merge so hotspot and lid mode cannot clobber each other
- Daemon restores missing `lid_mode` from prior state on startup
- Menu bar app uses unconditional KeepAlive and relaunches after unexpected exit
- Thermal watchdog notification after sustained non-nominal thermal or CPU > 90 °C

## 0.1.1

- Native menu copy
- About panel
- Screen-off-when-idle
- Config hardening
- Caffeinate no longer blocks display sleep

## 0.1.0

- First release
