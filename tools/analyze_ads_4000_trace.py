#!/usr/bin/env python3
"""Summarize read-only ADS_4000_TRACE diagnostics from debug.log."""
import argparse
import collections
import re
from pathlib import Path

TRACE = re.compile(
    r"ADS_4000_TRACE phase=(before|after) offset=(\d+) opcode=([0-9a-fA-F]{4}) "
    r"args=\[([^\]]*)\] skip=(-?\d+) running43=(-?\d+) running44=(-?\d+)"
)


def analyze(lines):
    pending = collections.defaultdict(list)
    counts = collections.Counter()
    changes = []
    unpaired_after = []
    starts = 0
    unpaired_before = []
    for line_no, line in enumerate(lines, 1):
        if "ADS_4000_TRACE test-mode-start" in line:
            starts += 1
            # A new invocation must not pair with records from an earlier run.
            unpaired_before.extend(n for items in pending.values() for n, _ in items)
            pending.clear()
        match = TRACE.search(line)
        if not match:
            continue
        phase, offset, opcode, args, skip, running43, running44 = match.groups()
        key = (offset, opcode, args)
        state = (int(skip), int(running43), int(running44))
        counts[opcode] += phase == "before"
        if phase == "before":
            pending[key].append((line_no, state))
        elif pending[key]:
            before_line, before_state = pending[key].pop()
            if state != before_state:
                changes.append((before_line, line_no, opcode, before_state, state))
        else:
            unpaired_after.append(line_no)
    unpaired_before.extend(line for items in pending.values() for line, _ in items)
    unpaired_before.sort()
    return starts, counts, changes, unpaired_before, unpaired_after


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("log", type=Path, help="Path to debug.log")
    args = parser.parse_args()
    starts, counts, changes, missing_after, missing_before = analyze(
        args.log.read_text(encoding="utf-8", errors="replace").splitlines()
    )
    print(f"Test-mode start markers: {starts}")
    print("Decoded instructions (before records):")
    for opcode, count in sorted(counts.items()):
        print(f"  0x{opcode}: {count}")
    print(f"State changes: {len(changes)}")
    for first, last, opcode, before, after in changes:
        print(f"  lines {first}-{last} opcode=0x{opcode}: {before} -> {after}")
    print(f"Unpaired before records: {len(missing_after)} at lines {missing_after}")
    print(f"Unpaired after records: {len(missing_before)} at lines {missing_before}")
    if not counts:
        print("No parseable instruction traces found; check build, mode and log location.")


if __name__ == "__main__":
    main()
