#!/usr/bin/env python3

"""
This program fetches the latest nightly build information for OpenShift versions 4.12-4.22
using the release-controller API. It shows the latest nightly version and how old it is.

Here are some examples of how to run it:
    python3 nightly-status.py
    python3 nightly-status.py --versions 4.15,4.16,4.17
    python3 nightly-status.py --json
    python3 nightly-status.py --verbose
"""

import argparse
import json
import re
import requests
import sys
from datetime import datetime, timezone
from typing import Dict, List, Optional, Tuple
from zoneinfo import ZoneInfo


def _parse_timestamp_from_tag(tag_name: str) -> Optional[datetime]:
    """
    Parse timestamp from nightly tag name.

    Arg(s):
        tag_name (str): Tag name like '4.15.0-0.nightly-2025-12-01-161151'
    Return Value(s):
        datetime: Parsed timestamp in UTC, or None if parsing fails
    """
    timestamp_match = re.search(r'(\d{4}-\d{2}-\d{2}-\d{6})', tag_name)
    if not timestamp_match:
        return None

    timestamp_str = timestamp_match.group(1)
    formatted_timestamp = f'{timestamp_str[:10]} {timestamp_str[11:13]}:{timestamp_str[13:15]}:{timestamp_str[15:17]}'

    try:
        return datetime.strptime(formatted_timestamp, '%Y-%m-%d %H:%M:%S').replace(tzinfo=timezone.utc)
    except ValueError:
        return None


def _calculate_age_string(tag_time: datetime) -> str:
    """
    Calculate human-readable age string from timestamp.

    Arg(s):
        tag_time (datetime): Timestamp of the nightly build
    Return Value(s):
        str: Human-readable age string like "2.6h" or "3d 4h"
    """
    now = datetime.now(timezone.utc)
    age = now - tag_time

    total_hours = age.total_seconds() / 3600
    days = age.days
    hours = age.seconds // 3600

    if days > 0:
        return f"{days}d {hours}h"
    else:
        return f"{total_hours:.1f}h"


def _fetch_latest_nightly_info(version: str) -> Optional[Tuple[str, str, str]]:
    """
    Fetch latest nightly information for a specific version.

    Arg(s):
        version (str): Version like '4.15'
    Return Value(s):
        tuple: (tag_name, phase, age_string) or None if failed
    """
    url = f"https://amd64.ocp.releases.ci.openshift.org/api/v1/releasestream/{version}.0-0.nightly/tags"

    try:
        response = requests.get(url, timeout=10)
        if response.status_code != 200:
            return None

        data = response.json()
        tags = data.get('tags', [])
        if not tags:
            return None

        latest_tag = tags[0]
        tag_name = latest_tag['name']
        phase = latest_tag['phase']

        tag_time = _parse_timestamp_from_tag(tag_name)
        if not tag_time:
            return (tag_name, phase, "unknown")

        age_string = _calculate_age_string(tag_time)
        return (tag_name, phase, age_string)

    except Exception:
        return None


def get_nightly_status(versions: List[str], show_progress: bool = False) -> Dict[str, dict]:
    """
    Get nightly status for multiple versions.

    Arg(s):
        versions (List[str]): List of versions like ['4.15', '4.16']
        show_progress (bool): Whether to show progress messages
    Return Value(s):
        dict: Dictionary with version as key and status info as value
    """
    results = {}

    for version in versions:
        if show_progress:
            print(f"Fetching {version}...", file=sys.stderr)
            sys.stderr.flush()

        info = _fetch_latest_nightly_info(version)

        if info:
            tag_name, phase, age_string = info
            results[version] = {
                'latest_nightly': tag_name,
                'phase': phase,
                'age': age_string,
                'available': True
            }
        else:
            results[version] = {
                'latest_nightly': 'N/A',
                'phase': 'N/A',
                'age': 'N/A',
                'available': False
            }

    return results


def print_table_output(results: Dict[str, dict]) -> None:
    """
    Print results in table format.

    Arg(s):
        results (dict): Results from get_nightly_status()
    Return Value(s):
        None
    """
    est_time = datetime.now(timezone.utc).astimezone(ZoneInfo('America/New_York'))
    run_time = est_time.strftime('%Y-%m-%d %H:%M:%S EST')
    print(f"\nRun time: {run_time}")
    print("\nOpenShift Nightly Build Status")
    print("=" * 60)
    print(f"{'Version':<8} {'Latest Nightly':<35} {'Phase':<10} {'Age':<8}")
    print("-" * 60)

    for version in sorted(results.keys(), key=lambda x: float(x), reverse=True):
        info = results[version]
        if info['available']:
            # Extract the full timestamp part for display (YYYY-MM-DD-HHMMSS)
            tag_parts = info['latest_nightly'].split('-')
            tag_display = f"{tag_parts[-4]}-{tag_parts[-3]}-{tag_parts[-2]}-{tag_parts[-1]}"
            print(f"{version:<8} {tag_display:<35} {info['phase']:<10} {info['age']:<8}")
        else:
            print(f"{version:<8} {'N/A':<35} {'N/A':<10} {'N/A':<8}")


def main() -> None:
    """
    Main function to parse arguments and run the nightly status check.

    Arg(s):
        None
    Return Value(s):
        None
    """
    parser = argparse.ArgumentParser(
        description="Check latest nightly build status for OpenShift versions 4.12-4.22"
    )
    parser.add_argument(
        "--versions",
        type=str,
        help="Comma-separated list of versions (e.g., 4.15,4.16). Default: 4.12-4.22"
    )
    parser.add_argument(
        "--json",
        action="store_true",
        help="Output results in JSON format"
    )
    parser.add_argument(
        "--verbose", "-v",
        action="store_true",
        help="Show progress messages while fetching"
    )

    args = parser.parse_args()

    if args.versions:
        versions = [v.strip() for v in args.versions.split(',')]
    else:
        versions = [f"4.{i}" for i in range(12, 23)]

    results = get_nightly_status(versions, show_progress=args.verbose)

    if args.json:
        est_time = datetime.now(timezone.utc).astimezone(ZoneInfo('America/New_York'))
        run_time = est_time.strftime('%Y-%m-%d %H:%M:%S EST')
        output = {
            'run_time': run_time,
            'results': results
        }
        print(json.dumps(output, indent=2))
    else:
        print_table_output(results)


if __name__ == "__main__":
    main()