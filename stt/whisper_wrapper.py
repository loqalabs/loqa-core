#!/usr/bin/env python3
"""Simple CLI wrapper around faster-whisper for Loqa."""

import argparse
import json
import math
import sys

try:
    from faster_whisper import WhisperModel
except ImportError as exc:  # pragma: no cover - import error messaging
    print(json.dumps({
        "error": "faster-whisper not installed",
        "detail": str(exc),
    }), file=sys.stderr)
    sys.exit(2)


def build_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description="Loqa faster-whisper wrapper")
    parser.add_argument("--model", required=True, help="Path to the Whisper model file")
    parser.add_argument("--audio", required=True, help="Path to the input WAV file")
    parser.add_argument("--language", default=None, help="Two-letter language code (optional)")
    parser.add_argument("--compute-type", default="int8", help="faster-whisper compute type (int8, float16, float32)")
    parser.add_argument("--beam-size", type=int, default=1)
    parser.add_argument("--temperature", type=float, default=0.0)
    parser.add_argument("--partial", action="store_true", help="Hint that this request is for an interim transcript")
    return parser.parse_args()


_model_cache = {}


def load_model(path: str, compute_type: str) -> WhisperModel:
    key = (path, compute_type)
    model = _model_cache.get(key)
    if model is None:
        model = WhisperModel(path, device="auto", compute_type=compute_type)
        _model_cache[key] = model
    return model


def main() -> int:
    args = build_args()
    model = load_model(args.model, args.compute_type)

    segments, info = model.transcribe(
        args.audio,
        language=args.language,
        beam_size=args.beam_size,
        temperature=args.temperature,
    )

    text = "".join(segment.text for segment in segments).strip()
    if not text:
        text = ""

    result = {
        "text": text,
    }
    # Convert avg_logprob (negative log probability, typically -1.0 to 0.0)
    # to a 0-1 confidence score using exponential transformation
    if info is not None and hasattr(info, "avg_logprob") and info.avg_logprob is not None:
        result["confidence"] = float(math.exp(info.avg_logprob))

    print(json.dumps(result))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
