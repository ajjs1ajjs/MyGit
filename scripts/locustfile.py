"""Locust load test for MyGit.

Usage:
    pip install locust
    locust -f scripts/locustfile.py --host http://127.0.0.1:8000
"""

import random

from locust import HttpUser, between, task


class MyGitUser(HttpUser):
    wait_time = between(1, 5)

    def on_start(self):
        self.username = f"loadtest_{random.randint(1, 100000)}"
        self.email = f"{self.username}@test.local"
        self.password = "testpass123"
        self.token = None
        self.repo_path = None
        self.register()

    def register(self):
        resp = self.client.post(
            "/api/v1/auth/register/",
            json={
                "username": self.username,
                "email": self.email,
                "password": self.password,
            },
        )
        if resp.status_code == 201:
            self.token = resp.json()["access"]

    @task(3)
    def list_projects(self):
        self.client.get("/api/v1/projects/", headers=self._auth)

    @task(2)
    def create_project(self):
        name = f"repo_{random.randint(1, 10000)}"
        resp = self.client.post(
            "/api/v1/projects/",
            json={
                "name": name,
                "visibility": "public",
            },
            headers=self._auth,
        )
        if resp.status_code == 201:
            self.repo_path = resp.json()["path"]

    @task(1)
    def git_clone(self):
        if not self.repo_path:
            return
        self.client.get(f"/{self.repo_path}.git/info/refs?service=git-upload-pack")

    @property
    def _auth(self):
        return {"Authorization": f"Bearer {self.token}"} if self.token else {}
