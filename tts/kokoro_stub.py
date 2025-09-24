#!/usr/bin/env python3
"""Stub TTS command for development. Converts text to silence."""

import base64
import json
import sys

def main() -> int:
    try:
        payload = json.load(sys.stdin)
    except json.JSONDecodeError as exc:
        print(json.dumps({"error": str(exc)}), file=sys.stderr)
        return 1

    sample_rate = int(payload.get("sample_rate", 22050))
    channels = int(payload.get("channels", 1))
    duration_sec = max(0.2, min(1.0, len(payload.get("text", "")) / 50.0))
    frame_count = int(sample_rate * duration_sec)
    pcm = bytearray(frame_count * channels * 2)

    resp = {
        "pcm_base64": base64.b64encode(pcm).decode("ascii"),
        "final": True,
    }
    print(json.dumps(resp))
    return 0

if __name__ == "__main__":
    raise SystemExit(main())
