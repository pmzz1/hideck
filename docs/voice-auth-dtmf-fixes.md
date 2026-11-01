# Voice, authentication and DTMF fixes

Based on upstream HiDeck v2.1.38, commit `087afaf0107c047cf46c45a697b6c3f6ba649edb`.
The combined changes were deployed as the local build `2.1.38+dtmf.1`.

## Early media and browser playback

Validated provisional IMS SDP now publishes a media update. Outbound calls
can attach early media while calling or ringing without marking the call
answered. Optional early attachment failure does not terminate the call;
incoming calls are not automatically attached.

The browser retries remote audio playback on `canplay` and track `unmute`.
An explicit playback button retries the current audio element without
redialling, replacing the media connection or requesting a microphone.
Browser autoplay and trusted HTTPS microphone requirements still apply.

## Separate ePDG and IMS authentication

Main and cellular startup previously wrapped the shared AKA provider with
the IMS application's preference. For a strict ISIM carrier, that also
forced ePDG authentication through ISIM and hid the provider's dedicated
ISIM interface from the runtime adapter.

Keep the original provider and let the runtime's existing authentication
plan choose USIM for ePDG and the configured application for IMS. This
preserves strict ISIM for IMS instead of weakening the carrier policy.

A US carrier profile that repeatedly failed ePDG AKA with card status
`9862` subsequently established its tunnel and registered IMS after this
change. No E911 address, SIM profile, device identity, proxy or encryption
proposal was changed as part of this fix.

## Wideband telephone-event negotiation

Phone-generated SDP previously advertised only `telephone-event/8000`,
even when offering 16 kHz speech codecs. Wideband SDP now also advertises
`telephone-event/16000`; narrowband SDP keeps its existing 8 kHz format.
RTP DTMF still uses only the format accepted in the remote SDP. A missing
negotiation is not bypassed by forcing an unaccepted payload type.

Safe diagnostic messages report the negotiated DTMF payload type and
clock rate, or the absence of telephone-event in the remote SDP. They do
not log digits, subscriber identities or full SDP.

The user confirmed that a new call's voice menu responds to keypad input
after deployment. This is a verified result for that environment, not a
claim of compatibility with every carrier or IVR.

## Validation

- Complete `internal/phone` tests, including early attachment, audio
  codecs and new wideband/narrowband DTMF offer tests, passed in an
  isolated Linux container with a private Docker network.
- Voice DTMF, SDP, provisional response, precondition, PRACK and reliable
  provisional response tests passed in the same environment.
- Complete `internal/sim` tests and runtimehost Reader/Prepare/AuthPlan
  tests passed. The new split-provider test verifies ePDG USIM and IMS
  ISIM entry points independently.
- All 395 frontend tests and the frontend typecheck/production build
  passed for the playback changes.
- On-device verification confirmed tunnel establishment and IMS
  registration. The user subsequently confirmed outbound calls and
  working IVR keypad input.

No router configuration, runtime database, logs, packet captures,
certificates, credentials or compiled deployment binaries are included.
Build the frontend and backend using the upstream build process; the
upstream Dockerfile already builds and embeds the frontend assets.
