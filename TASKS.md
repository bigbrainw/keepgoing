# keepgoing — task queue for Cursor agent

Phase 1 (lid mode) is DONE and committed. This is **Phase 2: publish v0.1.0**.
Read `README.md` first. Layout unchanged:

```
main.go                 CLI (daemon | install | uninstall | status | hotspot | lid | run | proxy | env)
internal/*              Go packages
app/main.swift          AppKit menu bar UI
app/Info.plist, app/build.sh   → dist/KeepGoing.app (Go daemon bundled as Contents/MacOS/keepgoing-cli)
```

## Hard rules (unchanged + additions)

- Build: `./app/build.sh`. Go binary in the bundle stays `keepgoing-cli`. Go 1.21 needs `-ldflags=-linkmode=external` + `codesign -s -` (build.sh does it).
- Never run `sudo`, `pmset`, `visudo`, write to `/etc`, or bounce Wi-Fi. Test daemons on `-listen 127.0.0.1:7790 -connect 127.0.0.1:7791 -no-wifi`.
- Deploy to this Mac = `rm -rf ~/Applications/KeepGoing.app && cp -R dist/KeepGoing.app ~/Applications/ && launchctl kickstart -k gui/$(id -u)/com.elijah.keepgoing && open ~/Applications/KeepGoing.app`.
- GitHub: `gh` is logged in as `bigbrainw`. Create the repo **private** (`gh repo create bigbrainw/keepgoing --private --source . --push`). Elijah flips it public. Releases as **draft**.
- No Apple Developer ID signing / notarisation in this phase (no cert yet). Ad-hoc only. Leave TODO hooks in the release script (`SIGN_IDENTITY`, `NOTARY_PROFILE` env vars: if set, use them; else ad-hoc).
- Commit after each task. Conventional Commits, subject ≤ 50 chars.

## Tasks (in order)

### 1. App icon
Generate `app/icon.iconset` → `app/AppIcon.icns` with a script (`app/make-icon.sh`) that draws a simple bolt on a rounded dark square using Swift + CoreGraphics or `sips`/`iconutil` from a 1024px PNG you render (no external tools, no downloads). Add `CFBundleIconFile` = `AppIcon` to `Info.plist`; build.sh copies the icns into `Contents/Resources/`. Verify: `open dist/KeepGoing.app` shows the icon in Finder Get Info.

### 2. Version plumbing
Single source of truth `VERSION` file (`0.1.0`). `build.sh` injects it into `Info.plist` (`CFBundleShortVersionString`, `CFBundleVersion`) and into Go via `-ldflags "-X main.version=..."`; add `keepgoing version` and put `"version"` into `/_status`. Menu shows `KeepGoing v0.1.0` as a disabled first item.

### 3. First-run onboarding (app/main.swift)
On first launch (no `~/.config/keepgoing/onboarded` marker): one NSAlert, 3 short paragraphs: what it does, "Keep running with lid closed" (one-time admin prompt), "Set hotspot". Buttons: **Set up lid mode** (runs the existing toggleLid flow), **Later**. Write the marker either way. Keep it to one alert — no wizard.

### 4. Release script `scripts/release.sh`
- Reads `VERSION`, runs `app/build.sh`, produces `dist/KeepGoing-<ver>.zip` (via `ditto -c -k --keepParent`) and `dist/KeepGoing-<ver>.dmg` (via `hdiutil create -volname KeepGoing -srcfolder <staging with app + Applications symlink> -ov -format UDZO`).
- If `SIGN_IDENTITY` is set: `codesign --deep --force --options runtime --timestamp -s "$SIGN_IDENTITY"`; if `NOTARY_PROFILE` set: `xcrun notarytool submit --keychain-profile "$NOTARY_PROFILE" --wait` + `xcrun stapler staple`. Else ad-hoc and print a warning.
- Prints SHA256 of both artifacts.

### 5. Install docs for unsigned build
README "Install" section at the top: download zip → unzip → drag to Applications → because it is not yet notarised: `xattr -d com.apple.quarantine /Applications/KeepGoing.app` (or right-click → Open). Also `keepgoing uninstall` + `sudo rm /etc/sudoers.d/keepgoing` for full removal. Add `LICENSE` (MIT, © 2026 Elijah).

### 6. Repo + draft release
`gh repo create bigbrainw/keepgoing --private --source . --push`. Tag `v0.1.0`. `gh release create v0.1.0 --draft --title "KeepGoing 0.1.0" --notes-file <generated from README top>` uploading the zip + dmg. Print the release URL.

### 7. Deploy to this Mac + smoke
Deploy per rule. `keepgoing status` shows `version`. Report: repo URL, draft release URL, artifact SHA256s, anything that failed.

## Backlog (do not start unless told)
- Sparkle auto-update; homebrew cask in a `bigbrainw/homebrew-tap`.
- Push notify (ntfy.sh) when offline > 3 min or hotspot join fails.
- Landing page (one static HTML) with the two GIFs: lid close + phone.
- Linux support.
