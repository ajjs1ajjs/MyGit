import json
import logging

from channels.db import database_sync_to_async
from channels.generic.websocket import AsyncWebsocketConsumer

from apps.ci_cd.models import Job

logger = logging.getLogger("mygit")


class JobLogConsumer(AsyncWebsocketConsumer):
    async def connect(self):
        user = self.scope.get("user")
        if not user or not user.is_authenticated:
            await self.close()
            return

        self.project_id = self.scope["url_route"]["kwargs"]["project_id"]
        self.pipeline_id = self.scope["url_route"]["kwargs"]["pipeline_id"]
        self.job_id = self.scope["url_route"]["kwargs"]["job_id"]
        self.room_group_name = f"job_{self.job_id}"

        job = await self.get_job()
        if not job:
            await self.close()
            return

        if not await self.can_access(user, job):
            await self.close()
            return

        # Join room group
        await self.channel_layer.group_add(self.room_group_name, self.channel_name)
        await self.accept()

        # Send initial log if job exists
        if job:
            await self.send(text_data=json.dumps({
                "type": "log",
                "data": job.log,
                "status": job.status,
            }))

    async def disconnect(self, close_code):
        await self.channel_layer.group_discard(self.room_group_name, self.channel_name)

    async def receive(self, text_data):
        data = json.loads(text_data)
        if data.get("type") == "ping":
            await self.send(text_data=json.dumps({"type": "pong"}))

    async def job_log(self, event):
        """Receive log update from channel layer and send to WebSocket."""
        await self.send(text_data=json.dumps({
            "type": "log",
            "data": event["data"],
            "status": event.get("status", ""),
        }))

    @database_sync_to_async
    def get_job(self):
        try:
            return Job.objects.select_related("pipeline__repository").get(
                id=self.job_id,
                pipeline_id=self.pipeline_id,
                pipeline__repository_id=self.project_id,
            )
        except Job.DoesNotExist:
            return None

    @database_sync_to_async
    def can_access(self, user, job) -> bool:
        from apps.repositories.models import Repository, RepositoryAccess

        repo = job.pipeline.repository
        if repo.visibility == Repository.Visibility.PUBLIC:
            return True
        if getattr(user, "is_superuser", False) or str(repo.owner_id) == str(user.id):
            return True
        return RepositoryAccess.objects.filter(
            repository=repo, user=user, role__gte=RepositoryAccess.Role.GUEST
        ).exists()
