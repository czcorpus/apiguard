import os
from typing import Any, Dict, List, TypedDict

import time
import datetime
import random
import json
import urllib
import urllib.request
import urllib.parse
import argparse

class Telemetry(TypedDict):
    actionName: str
    isMobile: bool
    isSubquery: bool
    tileName: str
    timestamp: int

class Conf(TypedDict):
    queries: List[str]
    actions: Dict[str, int]


def random_string(length: int) -> str:
    return ''.join(chr(random.randint(97, 123)) for _ in range(length))


def make_request(root_url: str, session_id: str, params_dict: Dict[str, Any]):
    print('Making request... ', end='')
    url = urllib.parse.urljoin(root_url, 'language-guide')
    params = urllib.parse.urlencode(params_dict)
    request = urllib.request.Request(f'{url}?{params}', headers = {'Cookie': f'wag.session={session_id}'})
    with urllib.request.urlopen(request) as response:
        response.read()
    print('Done')


def generate_telemetry(telemetry_actions: Dict[str, int], time_window_sec: int) -> List[Telemetry]:
    print('Generating telemetry... ', end='')
    telemetry: List[Telemetry] = []
    date = datetime.datetime.now()
    for action, count in telemetry_actions.items():
        for _ in range(count):
            telemetry.append({
                'actionName': action,
                'isMobile': False,
                'isSubquery': False,
                'tileName': '-',
                'timestamp': int(1000 * (date + random.random()*datetime.timedelta(seconds = time_window_sec)).timestamp())
            })
    print('Done')
    return telemetry


def send_telemetry(root_url: str, session_id: str, telemetry: List[Telemetry]):
    print('Sending telemetry... ', end='')
    url = urllib.parse.urljoin(root_url, 'telemetry')
    request = urllib.request.Request(
        url,
        headers = {'Content-Type': 'application/json', 'Cookie': f'wag.session={session_id}'},
        method = 'POST',
        data = json.dumps({'telemetry': telemetry}).encode()
    )
    with urllib.request.urlopen(request) as response:
        response.read()
    print('Done')


if __name__ == '__main__':
    parser = argparse.ArgumentParser()
    parser.add_argument("--root-url", help="middleware path", default='http://localhost:3010')
    parser.add_argument("--repeat", type=int, help="how many times to repeat", default=10)
    parser.add_argument("--telemetry-duration", type=int, help="duration of telemetry", default=2)
    parser.add_argument("--telemetry-conf", help="action parameters", default='telebot-conf.json')
    parser.add_argument("--session-id", help="action parameters", default=random_string(64))
    args = parser.parse_args()

    with open(os.path.join(os.path.dirname(__file__), args.telemetry_conf)) as f:
        conf: Conf = json.load(f)

    for i in range(args.repeat):
        print(f'Request {i+1}/{args.repeat}')
        make_request(args.root_url, args.session_id, {'q': random.choice(conf['queries'])})
        telemetry = generate_telemetry(conf['actions'], args.telemetry_duration)
        send_telemetry(args.root_url, args.session_id, telemetry)
        print(f'Waiting {args.telemetry_duration}s')
        time.sleep(args.telemetry_duration)
        print()
