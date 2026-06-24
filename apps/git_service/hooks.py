from pathlib import Path

PRE_RECEIVE = """#!/usr/bin/env python3
import json
import os
import sys
import urllib.request

API_URL = os.environ.get("MYGIT_API_URL", "http://127.0.0.1:8000")
API_TOKEN = os.environ.get("MYGIT_API_TOKEN", "")

def main():
    repo_path = os.environ.get("GL_REPO", "")
    if not repo_path:
        sys.exit(0)

    for line in sys.stdin:
        old_rev, new_rev, ref = line.strip().split(" ")
        data = json.dumps({
            "repo_path": repo_path,
            "old_rev": old_rev,
            "new_rev": new_rev,
            "ref": ref,
        }).encode()
        req = urllib.request.Request(
            f"{API_URL}/api/v1/internal/pre-receive",
            data=data,
            headers={"Content-Type": "application/json"},
        )
        if API_TOKEN:
            req.add_header("Authorization", f"Bearer {API_TOKEN}")
        try:
            resp = urllib.request.urlopen(req, timeout=10)
            if resp.status != 200:
                sys.exit(1)
        except urllib.error.HTTPError as e:
            msg = e.read().decode()
            print(msg, file=sys.stderr)
            sys.exit(1)
        except Exception:
            sys.exit(1)

if __name__ == "__main__":
    main()
"""

POST_RECEIVE = """#!/usr/bin/env python3
import json
import os
import sys
import urllib.request

API_URL = os.environ.get("MYGIT_API_URL", "http://127.0.0.1:8000")
API_TOKEN = os.environ.get("MYGIT_API_TOKEN", "")

def main():
    repo_path = os.environ.get("GL_REPO", "")
    if not repo_path:
        sys.exit(0)

    for line in sys.stdin:
        old_rev, new_rev, ref = line.strip().split(" ")
        data = json.dumps({
            "repo_path": repo_path,
            "old_rev": old_rev,
            "new_rev": new_rev,
            "ref": ref,
        }).encode()
        req = urllib.request.Request(
            f"{API_URL}/api/v1/internal/post-receive",
            data=data,
            headers={"Content-Type": "application/json"},
        )
        if API_TOKEN:
            req.add_header("Authorization", f"Bearer {API_TOKEN}")
        try:
            urllib.request.urlopen(req, timeout=10)
        except Exception:
            pass

if __name__ == "__main__":
    main()
"""


def install_hooks(repo_path: Path):
    hooks_dir = repo_path / "hooks"
    hooks_dir.mkdir(parents=True, exist_ok=True)

    pre_receive_path = hooks_dir / "pre-receive"
    pre_receive_path.write_text(PRE_RECEIVE)
    pre_receive_path.chmod(0o755)

    post_receive_path = hooks_dir / "post-receive"
    post_receive_path.write_text(POST_RECEIVE)
    post_receive_path.chmod(0o755)
