# Notification UX notes

Checked: 2026-09-25. Sources are the freedesktop notification spec, the GNOME and KDE HIGs, and current Plasma popup code. This is a research note for `attn`'s playback notification, not a spec.

## What attn does today

While speaking: summary is `🔊 <project>` (or `⚠` when `--alert`), body is the full message, buttons are Stop and Copy text, timeout is 0 (never expire), urgency is normal or critical, hints are `resident` and `suppress-sound`. KDE also gets `x-kde-origin-name`.

After playback, or after Stop: the same notification is replaced in place. Buttons are Replay, Close, and Copy text. Timeout becomes -1 (server default). The process keeps listening for `notify.linger` (default 15 minutes), then closes the notification.

A click on the popup chrome while speaking keeps the audio and drops the UI. Copy errors are ignored. If another message is already playing, the new call prints `Audio already playing, skipping.` and records history, and shows no notification.

## Spec points that bind the design

- Actions are optional. Clients should check the `actions` capability. The special key `default` is what a click on the notification itself invokes. [Protocol](https://specifications.freedesktop.org/notification-spec/latest/protocol.html)
- `replaces_id` must update the existing notification with no flicker. [Protocol](https://specifications.freedesktop.org/notification-spec/latest/protocol.html)
- `expire_timeout`: `-1` server default, `0` never expire, otherwise milliseconds. [Protocol](https://specifications.freedesktop.org/notification-spec/latest/protocol.html)
- `resident`: do not remove the notification when an action is invoked; the sender or the user removes it. [Hints](https://specifications.freedesktop.org/notification-spec/latest/hints.html)
- `suppress-sound`: set this when the client plays its own sound. [Hints](https://specifications.freedesktop.org/notification-spec/latest/hints.html)
- `transient`: bypass the server's persistence. [Hints](https://specifications.freedesktop.org/notification-spec/latest/hints.html)
- `desktop-entry`: the `.desktop` file prefix, used for the app icon and logging. [Hints](https://specifications.freedesktop.org/notification-spec/latest/hints.html)
- Urgency 2 (critical) should not auto-expire. Low and normal may. [Urgency levels](https://specifications.freedesktop.org/notification-spec/latest/urgency-levels.html)
- Hints are optional. A server may ignore any of them.

## HIG points that transfer

GNOME ([Notifications](https://developer.gnome.org/hig/patterns/feedback/notifications.html)):

- Notify about events the user needs while they are in another app. Do not rely on the notification as the only copy of the information.
- High-volume senders should throttle or summarize, and offer a way to turn notifications down.
- Remove a notification when it is no longer valid.
- The title alone should say what happened. Body is one extra sentence.
- Up to three action buttons. Use them for actions people often need. Do not duplicate the default (body-click) action.

KDE ([Communicating status changes](https://develop.kde.org/hig/status_changes/)):

- System notifications are for actionable events while the app is in the background: job progress, incoming messages, hardware problems.
- Do not notify about expected, ignorable events. Excessive notifications drive people off.
- Prefer not to send low urgency. Critical stays visible until dismissed. Normal notifications that must not be missed get the persistent flag.
- Silent failure is the worst way to report an error. Say what happened and how to proceed.

## Plasma, specifically

In current Plasma master, a popup timeout does not destroy a notification that is `resident`, or that has actions and is not `transient`. It only marks the popup expired, so the buttons stay usable in history. A non-resident notification with no actions is actually expired. Action clicks on a resident notification do not close it.

Source: [`applets/notifications/global/Globals.qml`](https://invent.kde.org/plasma/plasma-workspace/-/raw/master/applets/notifications/global/Globals.qml) `onExpired` and `onActionInvoked` (fetched 2026-09-25).

`attn` sets `resident` and always has actions, and does not set `transient`. On Plasma, the popup can fade and the history entry can keep Replay / Close / Copy text until our linger process closes it or exits. That behavior was read from Plasma source, not confirmed by clicking a live popup, and it was not checked on GNOME Shell. Our session treats any `NotificationClosed` as "stop listening", so a server that emits that signal when the banner times out would end the linger early.

## Cross-platform, not a Linux requirement

Windows allows up to five toast buttons, shown as quick actions that should not force the user out of their current task. [App notification content](https://learn.microsoft.com/en-us/windows/apps/develop/notifications/app-notifications/app-notifications-content). The Apple notifications page did not return readable text without JavaScript, so it is not cited here.
