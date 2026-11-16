# Changelog

What changed in each version of Kando, newest first. The app shows the section of a new version when it offers the update.

## 0.14.0 - 2026-11-16

### Features

- Type or search AI models, choose a whisper model file, more languages (#135)
- Open sites from Browse, say what Download Video did, pause hidden videos (#139)
- Search subtitle lines, read part of a film, open at a line, and make scripting safe (#134)
- Set up translation in the first-run guide and trim the AI and speech pages (#137)
- Study tools: pause after each line, multi-line loops, a list that follows the mode and lets the user read away (#140)
- Make the library, player and palette reachable by keyboard and readable by screen readers (#136)
- Add a playlist's or a channel's videos one job each, from the library, Browse, kando add and MCP (#144)
- Check speech model downloads against pinned checksums and offer hf-mirror.com (#143)
- Lead with the film being watched, show progress and spoken language on posters (#148)
- Explain Line, remedies under dead ends, subtitle style in Settings, and a selectable held line (#145)
- Keep films watchable through failures, show jobs everywhere, and pick the translation language per film (#138)
- Filter by spoken language in the sidebar and correct it per film (#146)
- Save lines, list them, export to Anki with audio, and review them (#147)
- Follow the system's light or dark appearance (#133)

### Fixes

- Open a film where it stopped from its poster, not at 0:00 (#125)
- Choose the new AI service's model, keep Test Connection current, stop on out-of-credit errors (#126)
- Make the palette's highlight match Return, add a palette button without a menu bar, and share shortcut labels and search (#128)
- Upload without a Google account, under kando/ when signed out (#130)
- Refuse pages that list videos, keep message bars current, search from Browse (#132)
- Truthful mode and clip notes, line-aligned clips, wider cue clock, finer speeds (#129)
- Keep a film watchable when Process Again fails, resume stopped translations, and say why Retry is refused (#127)
- Make subtitles of folder videos, keep them on failure, and add dropped folders (#142)

## 0.13.3 - 2026-11-16

### Fixes

- Use NSGo's windows and one instance on Windows and Linux, and check pixels on a platform (#122)

## 0.13.2 - 2026-11-15

No changes to the app worth listing: this version only carries maintenance.

## 0.13.1 - 2026-11-15

### Fixes

- Let Kando 0.12.0 and older update on macOS and Windows: this update replaces the MyGo build with the NSGo build of 0.13.0 and later, in the same place and with your libraries and settings, and later versions arrive through the new updater. On Linux the NSGo build needs GTK 4 and GStreamer, so it is not installed over the old one: remove the old copy with `curl -fsSL https://github.com/yldm-tech/Kando/releases/download/v0.12.0/install.sh | sh -s -- --uninstall` (your libraries and settings stay), then install the Debian or Arch package with `curl -fsSL https://github.com/yldm-tech/Kando/releases/latest/download/install.sh | sh`, or the AppImage on other distributions

## 0.13.0 - 2026-11-15

### Features

- Move Kando from MyGo to NSGo (#117)

## 0.12.1 - 2026-11-14

### Fixes

- Build on MyGo v0.9.0 and nodra v1.5.0, and quit without asking at logout or shutdown (#114)

## 0.12.0 - 2026-11-11

### Fixes

- Keep a tool kit's archive entries inside its folder on Windows too

## 0.11.2 - 2026-11-11

### Fixes

- Make the kando command, MCP redial and socket tests work on Windows
- Show the whole guide on Windows and fix Unix-only test assumptions
- Follow a renamed library on Windows
- Update a yt-dlp that is running

## 0.11.1 - 2026-11-11

No changes to the app worth listing: this version only carries maintenance.

## 0.11.0 - 2026-11-11

No changes to the app worth listing: this version only carries maintenance.

## 0.10.0 - 2026-11-10

No changes to the app worth listing: this version only carries maintenance.

## 0.9.2 - 2026-11-10

### Fixes

- Start a tool again while Linux says its file is busy

## 0.9.1 - 2026-11-10

### Fixes

- Keep a stale yt-dlp read from overwriting a newer one

## 0.9.0 - 2026-11-10

No changes to the app worth listing: this version only carries maintenance.

## 0.8.1 - 2026-11-10

### Fixes

- Say a video has no sound instead of showing ffmpeg's error

## 0.8.0 - 2026-11-10

### Features

- Drag subtitles up and down, off a film's burned-in lines

## 0.7.0 - 2026-11-10

### Fixes

- Queue a link typed without https:// instead of refusing it

## 0.6.0 - 2026-11-10

### Features

- Play sound on macOS, Linux and Windows through mygo/audio
- Add a built-in browser whose sign-ins carry into downloads

### Fixes

- Keep job records when another library is offline or library.json is damaged
- Stop the running job on quit, confirm quitting mid-job, keep Kando running when the main window closes on macOS
- Save the resume position while playing and remember subtitle delay per film
- Explain the everyapi CLI EveryAPI needs, and find it again without restarting
- Resume whisper model downloads and detect stalled downloads

## 0.5.1 - 2026-11-10

### Fixes

- Reject channel and list links, and do not queue the same video twice
- Use an existing .bilingual.srt beside a video that has no job record
- Esc leaves full screen first, and double-clicking the picture toggles full screen

## 0.5.0 - 2026-11-10

### Features

- Make the icon an amber glass speech bubble with a play mark
- Show updates in Settings › General

## 0.4.0 - 2026-11-09

### Features

- Check for updates from the app menu

## 0.3.0 - 2026-11-09

### Features

- Replace the app icon with a play mark over a pair of subtitle lines

## 0.2.0 - 2026-11-09

### Features

- Download yt-dlp, ffmpeg and whisper-cli on first launch
- Kando command line, MCP server and WebMCP

## 0.1.0 - 2026-11-09

### Breaking changes

- Switch config dir, library extension and env vars to Kando

### Features

- Scaffold native lingocut monorepo
- Add yt-dlp media pipeline boundary
- Connect native UI to video downloader
- Run download, transcription, translation and export pipeline
- Translate through EveryAPI by default with retries
- Keep processed videos in a sidebar library
- Restyle the library as a dark sidebar and film cards
- Import local videos with native file picker
- Implement audio playback for macOS and stub for other platforms
- Enhance library management and playback features
- Integrate nodra for session management and enhance library functionality
- Enhance icon handling and library logo functionality
- Enhance library renaming functionality and inode management
- Implement componentized UI structure using Onevra
- Integrate Google OAuth and Cloudflare R2 support
- Add AI logo integration and provider logo functionality
- Add Amazon S3 support alongside Cloudflare R2
- Add Alibaba Cloud OSS support for video uploads
- Add Tencent Cloud COS and S3-compatible storage support
- Phase 1 foundation: light theme, sub-states, shortcut table and page contracts
- Serial job queue, restart resume and library-switch stop (P1-2)
- Sidebar as a floating card with sources, library switcher and status line
- Library page in parts, drop-to-queue and remove to Trash (P1-3)
- Full-bleed player (P1-4)
- Settings window with categories, search and password fields (P1-5)
- Native menu bar and glass shortcut cheat sheet (P1-6)
- Add the feedback dialog that opens a prefilled GitHub issue
- Show real in-stage progress and the video's title on processing cards
- Classify yt-dlp failures and update yt-dlp the way it was installed
- Offer the remedy for each kind of failed download
- Add the ⌘K command palette
- First-run guide that opens 我的影片.kando, sets languages and downloads a speech model
- Pick the AI model from the service's own list, after the API key
- Animate side panel transitions
- Polish player panel and clip previews
- Add volume control and shortcuts
- List the sites yt-dlp downloads from, with their logos
- Use the video's own cover instead of a grabbed frame
- Interface in five languages, and each README in its own language

### Fixes

- Translate with the signed-in EveryAPI account
- Handle encoded local video paths
- Close the open film on a click of the current sidebar item and keep Mica on Windows
- Register the locales catalog so MyGo texts follow the bundle languages
- Center the upload capsule in the room the side panel leaves
- Keep the EveryAPI key off other LLM providers
- Stop the film's sound when the app ends
- Say 生成字幕 instead of 转录 for the speech stage
- Keep the source counts readable on the focused selection
- Tell yt-dlp where ffmpeg is so a Finder-launched app saves the sound
- Find browsers by the login folders yt-dlp reads, and offer Safari only on macOS
- Read yt-dlp's account-credentials login hint as a login, and an unresolvable host as its own kind
- Read browsers off the UI thread, offer no Safari fallback, and make 换个链接 match the prototype
- Scrub every storage credential and whole paths from the issue
- Keep yt-dlp partial files outside the library
- Retry failed batches, keep the film when some lines fail, say why in Chinese
- Keep the resume point when a film is only opened
- Follow a library renamed while the app was closed
- Picture beside the side panel, a top-right panel toggle, and room around the back button
- Restore missing translation language
- Recover short translation batches
- Separate back control from traffic lights
- Keep audio aligned after seeking
- Keep background playback smooth
- Keep picture and sound level when coming back to the app
- Join the sound once the window shows again

### Performance

- Speed up Whisper transcription
