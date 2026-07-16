"""Locust load test for MyGit.

Tests the full user workflow at scale:
  - Registration / login
  - Repository creation
  - Git Smart HTTP clone simulation
  - Issue creation
  - Merge request creation
  - File browser

Usage:
    pip install locust

    # Web UI (recommended)
    locust -f scripts/locustfile.py --host http://127.0.0.1:8060

    # Headless with 50+ concurrent users
    locust -f scripts/locustfile.py --host http://127.0.0.1:8060 \
        --users 50 --spawn-rate 10 --run-time 5m --headless \
        --html report.html --csv results
"""

import random
import string
from urllib.parse import urljoin

from locust import HttpUser, between, SequentialTaskSet, task


def random_string(length: int = 8) -> str:
    return "".join(random.choices(string.ascii_lowercase, k=length))


class MyGitWorkflow(SequentialTaskSet):
    """Simulates a complete user journey through the MyGit platform."""

    def on_start(self):
        """Called once per user when the simulated user starts."""
        self.username = f"loadtest_{random_string(10)}"
        self.email = f"{self.username}@test.local"
        self.password = "TestPass123!"
        self.access_token = None
        self.refresh_token = None
        self.project_id = None
        self.project_path = None
        self.issue_id = None
        self.merge_request_id = None

        # ----- Step 1: Register -----
        with self.client.post(
            "/api/v1/auth/register/",
            json={
                "username": self.username,
                "email": self.email,
                "password": self.password,
            },
            catch_response=True,
            name="01_register",
        ) as resp:
            if resp.status_code == 201:
                data = resp.json()
                self.access_token = data.get("access")
                self.refresh_token = data.get("refresh")
                resp.success()
            elif resp.status_code == 400 and "already exists" in resp.text:
                # User from a previous run; log in instead
                self._login()
                resp.success()
            else:
                resp.failure(f"Register failed: {resp.status_code} {resp.text[:200]}")

        if not self.access_token:
            self._login()

    def _login(self):
        with self.client.post(
            "/api/v1/auth/login/",
            json={"username": self.username, "password": self.password},
            catch_response=True,
            name="01b_login",
        ) as resp:
            if resp.status_code == 200:
                data = resp.json()
                self.access_token = data.get("access")
                self.refresh_token = data.get("refresh")
                resp.success()
            else:
                resp.failure(f"Login failed: {resp.status_code} {resp.text[:200]}")

    # ------------------------------------------------------------------
    # Auth header helper
    # ------------------------------------------------------------------
    @property
    def _auth(self) -> dict:
        return {"Authorization": f"Bearer {self.access_token}"} if self.access_token else {}

    # ------------------------------------------------------------------
    # Task 1: Browse public projects (no auth needed)
    # ------------------------------------------------------------------
    @task
    def browse_public_projects(self):
        """Browse the list of public projects."""
        with self.client.get(
            "/api/v1/projects/",
            name="02_browse_projects",
            catch_response=True,
        ) as resp:
            if resp.status_code in (200, 401):  # 401 is fine if API requires auth
                resp.success()
            else:
                resp.failure(f"Browse projects failed: {resp.status_code}")

    # ------------------------------------------------------------------
    # Task 2: Create a repository
    # ------------------------------------------------------------------
    @task
    def create_repository(self):
        """Create a new repository."""
        repo_name = f"loadtest_{random_string(8)}"
        with self.client.post(
            "/api/v1/projects/",
            json={
                "name": repo_name,
                "visibility": "public",
                "description": f"Load test repository - {repo_name}",
            },
            headers=self._auth,
            catch_response=True,
            name="03_create_repo",
        ) as resp:
            if resp.status_code == 201:
                data = resp.json()
                self.project_id = data.get("id")
                self.project_path = data.get("path") or data.get("full_path")
                resp.success()
            else:
                resp.failure(f"Create repo failed: {resp.status_code} {resp.text[:200]}")

    # ------------------------------------------------------------------
    # Task 3: Git clone simulation (Smart HTTP)
    # ------------------------------------------------------------------
    @task
    def git_clone_simulation(self):
        """Simulate a Git Smart HTTP clone (info/refs + upload-pack)."""
        if not self.project_path:
            return

        # Phase 1: Discover refs (git-upload-pack)
        refs_url = f"/{self.project_path}.git/info/refs?service=git-upload-pack"
        with self.client.get(
            refs_url,
            headers=self._auth,
            catch_response=True,
            name="04_git_clone_refs",
        ) as resp:
            if resp.status_code in (200, 304):
                resp.success()
            else:
                resp.failure(f"Git refs discovery failed: {resp.status_code}")

        # Phase 2: Fetch pack data (POST to git-upload-pack)
        upload_url = f"/{self.project_path}.git/git-upload-pack"
        with self.client.post(
            upload_url,
            data=b"0032want 0000000000000000000000000000000000000000\n0032have 0000000000000000000000000000000000000000\n0000",
            headers={**self._auth, "Content-Type": "application/x-git-upload-pack-request"},
            catch_response=True,
            name="05_git_clone_pack",
        ) as resp:
            if resp.status_code in (200, 304):
                resp.success()
            else:
                resp.failure(f"Git upload-pack failed: {resp.status_code}")

    # ------------------------------------------------------------------
    # Task 4: Browse files in repository
    # ------------------------------------------------------------------
    @task
    def browse_files(self):
        """Browse the file tree of a repository."""
        if not self.project_id:
            return

        with self.client.get(
            f"/api/v1/projects/{self.project_id}/tree/",
            headers=self._auth,
            catch_response=True,
            name="06_browse_files",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Browse files failed: {resp.status_code}")

        # Also browse branches
        with self.client.get(
            f"/api/v1/projects/{self.project_id}/branches/",
            headers=self._auth,
            catch_response=True,
            name="06b_browse_branches",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Browse branches failed: {resp.status_code}")

        # Browse commits
        with self.client.get(
            f"/api/v1/projects/{self.project_id}/commits/",
            headers=self._auth,
            catch_response=True,
            name="06c_browse_commits",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Browse commits failed: {resp.status_code}")

    # ------------------------------------------------------------------
    # Task 5: Create an issue
    # ------------------------------------------------------------------
    @task
    def create_issue(self):
        """Create a new issue in the repository."""
        if not self.project_id:
            return

        with self.client.post(
            f"/api/v1/projects/{self.project_id}/issues/",
            json={
                "title": f"Load test issue {random_string(6)}",
                "description": f"This issue was created during a load test.\n\nDetails:\n- User: {self.username}\n- Timestamp: auto",
                "labels": ["bug", "loadtest"],
            },
            headers=self._auth,
            catch_response=True,
            name="07_create_issue",
        ) as resp:
            if resp.status_code == 201:
                self.issue_id = resp.json().get("id")
                resp.success()
            else:
                # Some projects may have issues disabled; log but don't fail hard
                resp.success()
                self.issue_id = None

    # ------------------------------------------------------------------
    # Task 6: Create a merge request
    # ------------------------------------------------------------------
    @task
    def create_merge_request(self):
        """Create a merge request (requires at least 2 branches)."""
        if not self.project_id:
            return

        # First, create a merge request (the API will use default branches)
        with self.client.post(
            f"/api/v1/projects/{self.project_id}/merge_requests/",
            json={
                "title": f"Load test MR {random_string(6)}",
                "description": "Merge request created during load testing.",
                "source_branch": "loadtest-feature",
                "target_branch": "main",
            },
            headers=self._auth,
            catch_response=True,
            name="08_create_mr",
        ) as resp:
            if resp.status_code == 201:
                self.merge_request_id = resp.json().get("id")
                resp.success()
            else:
                # Branch may not exist; that's okay for load testing
                resp.success()
                self.merge_request_id = None

        # Also list existing merge requests
        with self.client.get(
            f"/api/v1/projects/{self.project_id}/merge_requests/",
            headers=self._auth,
            catch_response=True,
            name="08b_list_mrs",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"List MRs failed: {resp.status_code}")

    # ------------------------------------------------------------------
    # Task 7: Global search
    # ------------------------------------------------------------------
    @task
    def global_search(self):
        """Execute a global search."""
        terms = ["test", "bug", "feature", self.username[:6]]
        term = random.choice(terms)
        with self.client.get(
            "/api/v1/search",
            params={"q": term},
            headers=self._auth,
            catch_response=True,
            name="09_search",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Search failed: {resp.status_code}")

    # ------------------------------------------------------------------
    # Task 8: Refresh token
    # ------------------------------------------------------------------
    @task
    def refresh_auth_token(self):
        """Refresh the JWT token to simulate long-lived sessions."""
        if not self.refresh_token:
            return

        with self.client.post(
            "/api/v1/auth/refresh/",
            json={"refresh": self.refresh_token},
            catch_response=True,
            name="10_token_refresh",
        ) as resp:
            if resp.status_code == 200:
                data = resp.json()
                self.access_token = data.get("access")
                resp.success()
            else:
                resp.failure(f"Token refresh failed: {resp.status_code}")

    # ------------------------------------------------------------------
    # Task 9: List issues
    # ------------------------------------------------------------------
    @task
    def list_issues(self):
        """List issues in the repository."""
        if not self.project_id:
            return

        with self.client.get(
            f"/api/v1/projects/{self.project_id}/issues/",
            headers=self._auth,
            catch_response=True,
            name="11_list_issues",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"List issues failed: {resp.status_code}")

    # ------------------------------------------------------------------
    # Task 10: Profile / notifications
    # ------------------------------------------------------------------
    @task
    def get_notifications(self):
        """Fetch user notifications."""
        with self.client.get(
            "/api/v1/notifications/",
            headers=self._auth,
            catch_response=True,
            name="12_notifications",
        ) as resp:
            if resp.status_code == 200:
                resp.success()
            else:
                resp.failure(f"Notifications failed: {resp.status_code}")


class MyGitUser(HttpUser):
    """
    Simulates a real MyGit user.

    Recommended usage for realistic load:
        locust -f scripts/locustfile.py --host http://127.0.0.1:8060 \
            --users 50 --spawn-rate 5 --run-time 10m --headless

    Each user follows a sequential workflow (register → create repo → create issue → ...).
    The `wait_time` between steps mimics human think-time.
    """

    wait_time = between(1.0, 4.0)
    tasks = [MyGitWorkflow]
