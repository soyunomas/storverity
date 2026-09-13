# Accessibility and destructive-operation UX review

This review is part of Phase 5 and applies to the Wails/Svelte desktop application.

## Accessibility review

StorVerity uses native keyboard-focusable `button`, `select` and `input` controls for primary interactions. Device refresh has an accessible name, decorative icons are hidden from assistive technology, progress/result messages use live/status semantics where appropriate, and destructive confirmation uses a labelled text input rather than pointer-only confirmation.

The Phase 5 pass adds a clearly visible `:focus-visible` treatment for interactive controls and honors `prefers-reduced-motion`. Region cells expose text labels in addition to color/state styling so corruption and I/O outcomes are not color-only information. Error messages use alert semantics, while save/completion messages use polite status semantics.

The UI remains optimized for a desktop viewport (minimum Wails window size 900×620). Keyboard access and focus visibility are release requirements; full screen-reader testing across multiple Linux desktop environments remains an ongoing compatibility task rather than a one-time claim of conformance to a specific WCAG level.

## Destructive-operation review

The raw capacity probe is intentionally harder to start than filesystem verification:

1. The selected device must already pass the conservative raw-test safety policy.
2. The UI presents a persistent data-loss warning and identifies raw probing as direct block writes.
3. Preparation creates a short-lived one-use challenge bound to the selected device identity.
4. The user must type the exact generated confirmation phrase containing the target device path.
5. The backend refreshes discovery immediately before open, validates the opened Linux `major:minor`, holds the target exclusively and refreshes safety once more before the first write.
6. A running probe retains an explicit Stop-and-restore control even if the visible selection changes.
7. Cancellation stops new writes but does not skip best-effort restoration of blocks already touched.
8. The result distinguishes corruption/I/O failures from restoration failures and can be exported as a report.

StorVerity never describes sampled raw probing as non-destructive. The UI states that restoration can fail after power loss, disconnects, fraudulent firmware or I/O errors and directs users to expendable media.

## Release blockers

The following are release blockers for destructive UX changes:

- any path that allows a free-form `/dev/...` value from the frontend to authorize raw I/O;
- removing the exact confirmation phrase without an equivalent or stronger explicit confirmation mechanism;
- hiding restoration failures inside a generic success state;
- allowing a mounted/system/swap/read-only/non-external target through the safety gate;
- removing keyboard access to Start/Stop/report controls;
- conveying raw-probe failure solely through color.
