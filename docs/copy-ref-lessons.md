# Lessons: `blf copy-ref` (macOS clipboard file references)

Hard-won notes from implementing `blf copy-ref` on macOS. The Linux (`wl-copy`)
path was straightforward; everything below is about the macOS pasteboard.

## 1. `set the clipboard to {POSIX file ...}` cannot copy multiple files

The plan started with AppleScript:

```applescript
set the clipboard to {POSIX file "/a.png", POSIX file "/b.png"}
```

AppleScript can only coerce a **single** value into a `furl` (file-URL)
pasteboard flavor. A *list* of `POSIX file`s becomes an opaque AppleScript list
on the clipboard (`clipboard info` shows `list, 130` instead of `«class furl»`).
No GUI app can paste that. The single-file form (`set the clipboard to POSIX
file "..."`) works, which is exactly why it's easy to ship the bug — the simplest
smoke test passes.

**Correct API:** `NSPasteboard.writeObjects` with an array of `NSURL`s, driven
from JXA (`osascript -l JavaScript`). This produces real file references for both
single and multiple files. No new Go dependency — still one shelled-out tool.

## 2. JXA: `$(jsArray)` silently writes only the FIRST object

This looked right but only copied one file:

```javascript
var urls = paths.map(p => $.NSURL.fileURLWithPath(p));
pb.writeObjects($(urls));   // BUG: writes only urls[0]
```

Bridging a JS array to `NSArray` via `$(...)` does **not** hand a proper
`NSArray` of `NSURL`s to `writeObjects`. Build an explicit `NSMutableArray`:

```javascript
var urls = $.NSMutableArray.alloc.init;
paths.forEach(p => urls.addObject($.NSURL.fileURLWithPath(p)));
pb.writeObjects(urls);      // writes all of them
```

## 3. `writeObjects([NSURL...])` is RACY from a short-lived CLI process

Even after building a proper `NSMutableArray` of `NSURL`s (lesson #2),
`pb.writeObjects(arr)` was **flaky**: copying 3 files landed 1, 2, or 3 of them
at random across runs.

Cause: `writeObjects` creates **one pasteboard item per object, with lazy data
providers** that resolve *after* the call returns. A CLI process exits the
instant `osascript` returns, so the providers race against process teardown and
a random subset survives. (Run by hand it usually works — the shell keeps things
alive a moment longer — so this hides during interactive testing and only bites
the real binary. See lesson #5.)

Confirmed it's the lazy providers, not the write, by isolating the race:
- write then read back **in the same process** → always all N.
- write, then read back in a **separate process** → random subset.

Fix (chosen): keep the modern API but **force the lazy data to materialize
in-process** before exit, by reading each item's `public.file-url`:

```javascript
ObjC.import('AppKit');
var paths = [...];
var urls = $.NSMutableArray.alloc.init;
paths.forEach(p => urls.addObject($.NSURL.fileURLWithPath(p)));
var pb = $.NSPasteboard.generalPasteboard;
pb.clearContents;
pb.writeObjects(urls);
// force lazy providers to resolve synchronously before the process exits
var items = pb.pasteboardItems;
for (var i = 0; i < items.count; i++) {
  items.objectAtIndex(i).dataForType('public.file-url');
}
```

Verified 30/30 runs land all 3 files. A hardcoded `NSThread.sleepForTimeInterval`
also worked (20/20) but is an arbitrary guess; the force-read is deterministic.

Alternative considered: the legacy **`NSFilenamesPboardType`** (a single plist
array of paths, written eagerly — no per-item provider, so no race; 25/25). It's
the long-standing multi-file convention apps read, but it's been
`API_DEPRECATED` since macOS 10.14 ("Create multiple pasteboard items with
NSPasteboardTypeFileURL instead"), so we preferred the modern path. No removal
date is announced; it still ships in the macOS 26.5 SDK.

Detecting the race: a count that **varies run-to-run** is the signature — loop
the clear→run→readback 20+ times rather than trusting a single run.

## 4. `clipboard info` is a USELESS verifier for this

`osascript -e 'clipboard info'` only reports the flavors of the **first**
pasteboard item. It shows `«class furl», 35` whether one file or ten are on the
clipboard — so it cannot distinguish "copied 1" from "copied N". It gave a false
green twice.

**Reliable verification** — count items and read each `public.file-url`:

```javascript
ObjC.import('AppKit');
var pb = $.NSPasteboard.generalPasteboard;
var items = pb.pasteboardItems;
var n = items.count, out = [];
for (var i = 0; i < n; i++) {
  var u = $.NSURL.URLWithString(items.objectAtIndex(i).stringForType('public.file-url'));
  out.push(ObjC.unwrap(u.path));
}
JSON.stringify({count: '' + n, paths: out});
```

Note: `readObjectsForClassesOptions([$.NSURL], $())` — passing a JS array of
classes — throws "does not implement the NSPasteboardReading protocol". Read the
`pasteboardItems` directly (above) instead.

## 5. A stale binary masked the fix (the big time-sink)

After editing the script I kept seeing `count: 1`. The edit was correct — but
`go run .` was serving a **cached compile** from before the change, so it kept
running the old broken script. The same script run by hand via `osascript` gave
`count: 2`; through the binary it gave `count: 1`. That divergence is the tell.

Lessons:
- When a fix "doesn't take," suspect the build cache. Force `go build -a` (or
  build to an explicit path and run that exact file) before concluding the code
  is wrong.
- Verify with an **atomic** sequence in one shell invocation —
  clear → run binary → read back — so you never measure a clipboard left by an
  earlier ad-hoc `osascript` test instead of by the binary.

## 6. Embed paths as a JSON array, not hand-rolled escaping

`json.Marshal(paths)` produces a JSON array that is also a valid JavaScript
array literal, with correct escaping of quotes, backslashes, and unicode. This
replaced a hand-written AppleScript escaper and handles nasty filenames
(spaces, embedded `"`, `\`) for free.

## 7. Testing seam held up well

Making the OS a parameter (`buildCopyRefCommand(goos, paths)`) and routing exec
through the `runCommand` dep meant the command-construction logic was unit-tested
from the Mac for both platforms. But note the limit this exposed: those unit
tests assert the *script text*, and the script text was correct all along — the
bugs (#1, #2) lived in runtime pasteboard semantics that only a real round-trip
catches. Pure-function tests verify construction; they do not verify that the OS
does what you think with the result. Keep a real round-trip check for the exec.
