"""Unit tests for ADS trace log pairing; no original assets required."""
import importlib.util
from pathlib import Path
import unittest

SCRIPT = Path(__file__).with_name("analyze_ads_4000_trace.py")
spec = importlib.util.spec_from_file_location("ads_trace_analyzer", SCRIPT)
module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(module)


def record(phase, offset=12, opcode="4000", skip=0):
    return (f"ADS_4000_TRACE phase={phase} offset={offset} opcode={opcode} "
            f"args=[3 44 1 0] skip={skip} running43=0 running44=0")


class TraceTests(unittest.TestCase):
    def test_pair_and_change(self):
        starts, counts, changes, before, after = module.analyze([
            "ADS_4000_TRACE test-mode-start ads=BUILDING.ADS tag=7",
            record("before"), record("after", skip=1)
        ])
        self.assertEqual((starts, counts["4000"]), (1, 1))
        self.assertEqual(len(changes), 1)
        self.assertEqual((before, after), ([], []))

    def test_nested_matching_is_lifo(self):
        _, _, changes, before, after = module.analyze([
            record("before", skip=0),
            record("before", skip=1),
            record("after", skip=1),
            record("after", skip=0),
        ])
        self.assertEqual((changes, before, after), ([], [], []))

    def test_new_session_does_not_pair_old_record(self):
        _, _, _, before, after = module.analyze([
            record("before"),
            "ADS_4000_TRACE test-mode-start ads=BUILDING.ADS tag=8",
            record("after"),
        ])
        self.assertEqual(before, [1])
        self.assertEqual(after, [3])

    def test_missing_records(self):
        _, counts, _, before, after = module.analyze([
            record("after", offset=10),
            record("before", offset=11),
        ])
        self.assertEqual(counts["4000"], 1)
        self.assertEqual((before, after), ([2], [1]))


if __name__ == "__main__":
    unittest.main()
