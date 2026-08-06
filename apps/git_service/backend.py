import os
import subprocess
from pathlib import Path
from typing import Any, Optional

from django.conf import settings
from git import Repo as GitPythonRepo

from .hooks import install_hooks

SERVICE_MAP = {
    "git-upload-pack": "upload-pack",
    "git-receive-pack": "receive-pack",
}

GIT_BINARY = getattr(settings, "GIT_BINARY", "git")


class GitServiceError(Exception):
    pass


def git_env(repo_logical_path: str = "") -> dict:
    """Environment for git subprocesses.

    Carries the logical repository path (used by pre/post-receive hooks) and the
    internal API token so that hooks can authenticate against the internal API.
    """
    env = os.environ.copy()
    if repo_logical_path:
        env["GL_REPO"] = repo_logical_path
    token = getattr(settings, "MYGIT_INTERNAL_API_TOKEN", "")
    if token:
        env["MYGIT_INTERNAL_API_TOKEN"] = token
    return env


class GitBackend:
    def __init__(self, repo_path: Path):
        self.repo_path = repo_path

    def init_bare(self) -> GitPythonRepo:
        repo = GitPythonRepo.init(self.repo_path, bare=True)
        install_hooks(self.repo_path)
        repo.close()
        return repo

    def exists(self) -> bool:
        return self.repo_path.exists()

    def get_repo(self) -> GitPythonRepo:
        return GitPythonRepo(self.repo_path)

    def get_branches(self) -> list[dict]:
        repo = self.get_repo()
        try:
            branches = []
            for head in repo.heads:
                branches.append({"name": head.name, "sha": head.commit.hexsha})
            return branches
        finally:
            repo.close()

    def get_tags(self) -> list[dict]:
        repo = self.get_repo()
        try:
            tags = []
            for tag in repo.tags:
                tags.append({"name": tag.name, "sha": str(tag.commit)})
            return tags
        finally:
            repo.close()

    def get_default_branch(self) -> str:
        repo = self.get_repo()
        try:
            if repo.heads:
                return repo.active_branch.name
            return "main"
        finally:
            repo.close()

    def delete(self) -> None:
        import shutil

        shutil.rmtree(self.repo_path)

    def handle_smart_http(self, service: str, input_stream: Optional[bytes] = None) -> bytes:
        if service not in SERVICE_MAP:
            raise GitServiceError(f"Unknown service: {service}")

        git_service = SERVICE_MAP[service]
        cmd = [GIT_BINARY, git_service, "--stateless-rpc"]
        if input_stream is None:
            cmd.append("--advertise-refs")
        cmd.append(str(self.repo_path))

        proc = subprocess.Popen(
            cmd,
            stdin=subprocess.PIPE if input_stream else None,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=git_env(),
        )
        try:
            stdout, stderr = proc.communicate(input=input_stream, timeout=120)
        except subprocess.TimeoutExpired:
            proc.kill()
            proc.communicate()
            raise GitServiceError("Git process timed out.")

        if proc.returncode != 0:
            raise GitServiceError(stderr.decode(errors="replace"))

        return stdout

    def stream_smart_http(
        self, service: str, input_stream=None, repo_logical_path: str = ""
    ):
        """Stream a stateless-rpc request (push/pull) without buffering the body.

        ``input_stream`` must be a file-like object (Django ``request.stream``).
        Returns a generator yielding stdout chunks.
        """
        import shutil

        if service not in SERVICE_MAP:
            raise GitServiceError(f"Unknown service: {service}")

        git_service = SERVICE_MAP[service]
        cmd = [GIT_BINARY, git_service, "--stateless-rpc", str(self.repo_path)]
        proc = subprocess.Popen(
            cmd,
            stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            env=git_env(repo_logical_path),
        )

        def generate():
            try:
                if input_stream is not None:
                    shutil.copyfileobj(input_stream, proc.stdin, length=64 * 1024)
            except Exception:
                proc.kill()
            finally:
                try:
                    proc.stdin.close()
                except Exception:
                    pass
            try:
                while True:
                    chunk = proc.stdout.read(64 * 1024)
                    if not chunk:
                        break
                    yield chunk
            finally:
                if proc.stdout:
                    proc.stdout.close()
                if proc.stderr:
                    proc.stderr.close()
                proc.wait()

        return generate()

    def has_commits(self) -> bool:
        repo = self.get_repo()
        try:
            return bool(repo.heads)
        except Exception:
            return False
        finally:
            repo.close()

    def get_tree(self, ref: str = "HEAD", path: str = "") -> list[dict]:
        if not self.has_commits():
            return []
        repo = self.get_repo()
        try:
            commit = repo.commit(ref)
            tree = commit.tree
            if path:
                for part in path.strip("/").split("/"):
                    if part:
                        tree = tree[part]

            entries = []
            for entry in tree:
                entry_type = "tree" if entry.type == "tree" else "blob"
                entries.append(
                    {
                        "name": entry.name,
                        "type": entry_type,
                        "mode": entry.mode,
                        "sha": entry.hexsha,
                        "path": f"{path}/{entry.name}".strip("/"),
                    }
                )
            return entries
        finally:
            repo.close()

    def get_blob(self, sha: str) -> tuple[str, str]:
        repo = self.get_repo()
        try:
            data = repo.git.show(sha)
            return data, sha
        finally:
            repo.close()

    def get_blob_by_path(self, ref: str, path: str) -> tuple[str, str]:
        repo = self.get_repo()
        try:
            commit = repo.commit(ref)
            blob = commit.tree / path
            return blob.data_stream.read().decode("utf-8", errors="replace"), blob.hexsha
        finally:
            repo.close()

    def get_commits(self, ref: str = "HEAD", page: int = 1, per_page: int = 20) -> list[dict]:
        if not self.has_commits():
            return []
        repo = self.get_repo()
        try:
            repo.commit(ref)
            commits: list[dict] = []
            skip = (page - 1) * per_page
            max_count = skip + per_page
            for c in list(repo.iter_commits(ref, max_count=max_count))[skip:]:
                commits.append(
                    {
                        "sha": c.hexsha,
                        "short_sha": c.hexsha[:8],
                        "message": c.message.strip(),
                        "author": {"name": c.author.name, "email": c.author.email},
                        "committed_at": c.committed_datetime.isoformat(),
                        "parents": [p.hexsha for p in c.parents],
                    }
                )
            return commits
        finally:
            repo.close()

    def get_commit(self, sha: str) -> dict | None:
        if not self.has_commits():
            return None
        repo = self.get_repo()
        try:
            c = repo.commit(sha)
            return {
                "sha": c.hexsha,
                "short_sha": c.hexsha[:8],
                "message": c.message.strip(),
                "author": {"name": c.author.name, "email": c.author.email},
                "committer": {"name": c.committer.name, "email": c.committer.email},
                "committed_at": c.committed_datetime.isoformat(),
                "authored_at": c.authored_datetime.isoformat(),
                "parents": [p.hexsha for p in c.parents],
                "stats": c.stats.total if hasattr(c, "stats") else {},
            }
        finally:
            repo.close()

    def get_commit_diff(self, sha: str) -> list[dict]:
        repo = self.get_repo()
        try:
            commit = repo.commit(sha)
            diffs: list[dict] = []
            for d in commit.diff(commit.parents[0] if commit.parents else None):
                raw_diff = d.diff
                if isinstance(raw_diff, bytes):
                    raw_diff = raw_diff.decode("utf-8", errors="replace")
                diffs.append(
                    {
                        "type": d.change_type,
                        "old_path": d.a_path,
                        "new_path": d.b_path,
                        "diff": raw_diff or "",
                    }
                )
            return diffs
        finally:
            repo.close()

    def get_blame(self, ref: str, path: str) -> list[dict]:
        repo = self.get_repo()
        try:
            blame_lines: list[dict] = []
            raw_blame = repo.blame(ref, path)
            if raw_blame is None:
                return blame_lines
            for entry in raw_blame:
                c: Any = entry[0]
                lines = entry[1] if len(entry) > 1 else []
                if lines is None:
                    continue
                for line in lines:
                    line_str = (
                        line.decode("utf-8", errors="replace")
                        if isinstance(line, bytes)
                        else str(line)
                    )
                    blame_lines.append(
                        {
                            "sha": c.hexsha,
                            "author": c.author.name,
                            "author_email": c.author.email,
                            "committed_at": c.committed_datetime.isoformat(),
                            "line": line_str,
                        }
                    )
            return blame_lines
        finally:
            repo.close()

    def create_branch(self, name: str, ref: str = "HEAD") -> dict:
        repo = self.get_repo()
        try:
            commit = repo.commit(ref)
            new_branch = repo.create_head(name, commit)
            return {"name": new_branch.name, "sha": new_branch.commit.hexsha}
        finally:
            repo.close()

    def delete_branch(self, name: str) -> None:
        repo = self.get_repo()
        try:
            repo.delete_head(name)
        finally:
            repo.close()

    def create_tag(self, name: str, ref: str = "HEAD", message: str = "") -> dict:
        repo = self.get_repo()
        try:
            commit = repo.commit(ref)
            if message:
                tag = repo.create_tag(name, ref=commit, message=message)
            else:
                tag = repo.create_tag(name, ref=commit)
            return {"name": tag.name, "sha": str(tag.commit)}
        finally:
            repo.close()

    def delete_tag(self, name: str) -> None:
        repo = self.get_repo()
        try:
            repo.delete_tag(repo.tags[name])
        finally:
            repo.close()

    def get_merge_request_diff(self, target: str, source: str) -> list[dict]:
        repo = self.get_repo()
        try:
            diffs: list[dict] = []
            for d in repo.commit(source).diff(target):
                raw_diff = d.diff
                if isinstance(raw_diff, bytes):
                    raw_diff = raw_diff.decode("utf-8", errors="replace")
                diffs.append(
                    {
                        "type": d.change_type,
                        "old_path": d.a_path,
                        "new_path": d.b_path,
                        "diff": raw_diff or "",
                    }
                )
            return diffs
        finally:
            repo.close()

    def fast_forward_merge(self, source: str, target: str) -> dict:
        repo = self.get_repo()
        try:
            merge_base = repo.merge_base(source, target)
            target_commit = repo.commit(target)
            if not merge_base or merge_base[0] != target_commit:
                raise GitServiceError(
                    "Cannot fast-forward: target branch is not an ancestor of source."
                )

            source_commit = repo.commit(source)
            repo.head.reference = repo.heads[target]
            repo.head.reference.commit = source_commit
            repo.head.reset(index=True, working_tree=False)
            return {"sha": source_commit.hexsha, "method": "fast-forward"}
        finally:
            repo.close()

    def merge_commit(self, source: str, target: str, message: str) -> dict:
        import subprocess as sp

        merge_msg = f"Merge branch '{source}' into {target}\n\n{message}"
        cmd = [
            GIT_BINARY,
            "-C",
            str(self.repo_path),
            "merge",
            "--no-ff",
            "--no-edit",
            "-m",
            merge_msg,
            source,
        ]
        merge_env = git_env()
        merge_env.update({"GIT_EDITOR": "true", "GIT_MERGE_AUTOEDIT": "no"})
        existing = sp.run(
            ["git", "-C", str(self.repo_path), "rev-parse", target],
            capture_output=True,
            text=True,
        )
        if existing.returncode != 0:
            raise GitServiceError(f"Target branch '{target}' not found.")

        proc = sp.run(cmd, capture_output=True, text=True, env=merge_env)
        if proc.returncode != 0:
            error = proc.stderr or "Merge conflict or error."
            raise GitServiceError(error)

        sha = sp.run(
            ["git", "-C", str(self.repo_path), "rev-parse", "HEAD"],
            capture_output=True,
            text=True,
        ).stdout.strip()
        return {"sha": sha, "method": "merge-commit"}

    def get_archive_stream(self, ref: str = "HEAD", fmt: str = "tar.gz"):
        import subprocess as sp

        if fmt not in ("tar.gz", "zip", "tar"):
            raise GitServiceError(f"Unsupported format: {fmt}")
        cmd = [GIT_BINARY, "archive", f"--format={fmt}", "--output=-", ref]

        def generate():
            proc = sp.Popen(cmd, stdout=sp.PIPE, stderr=sp.PIPE, cwd=str(self.repo_path), env=git_env())
            stderr_data = b""
            try:
                while True:
                    chunk = proc.stdout.read(65536)
                    if not chunk:
                        break
                    yield chunk
            finally:
                stderr_data = proc.stderr.read()
                proc.stdout.close()
                proc.stderr.close()
                proc.wait()
                if proc.returncode != 0:
                    raise GitServiceError(stderr_data.decode(errors="replace"))

        return generate()

    @staticmethod
    def get_repo_size_kb(repo_path: Path) -> int:
        import subprocess as sp

        try:
            result = sp.run(
                [GIT_BINARY, "-C", str(repo_path), "count-objects", "-v"],
                capture_output=True,
                text=True,
                timeout=30,
            )
            for line in result.stdout.splitlines():
                if "size-pack" in line:
                    return int(line.split(":")[1].strip()) * 1024  # convert MiB to KiB
        except Exception:
            pass

        total_size = 0
        for dirpath, _, filenames in os.walk(repo_path):
            for f in filenames:
                fp = os.path.join(dirpath, f)
                try:
                    total_size += os.path.getsize(fp)
                except OSError:
                    pass
        return total_size // 1024
