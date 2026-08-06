from django.db.models import Q
from django.shortcuts import get_object_or_404
from rest_framework import status, viewsets
from rest_framework.decorators import action
from rest_framework.exceptions import PermissionDenied
from rest_framework.permissions import IsAuthenticated
from rest_framework.response import Response

from apps.repositories.models import Repository

from .models import Group, GroupMember, Team, TeamMembership
from .serializers import (
    GroupDetailSerializer,
    GroupListSerializer,
    GroupMemberSerializer,
    TeamMembershipSerializer,
    TeamSerializer,
)


class GroupViewSet(viewsets.ModelViewSet):
    permission_classes = [IsAuthenticated]
    lookup_field = "id"

    def get_serializer_class(self):
        if self.action in ("members", "add_member"):
            return GroupMemberSerializer
        if self.action in ("teams", "team_members"):
            return TeamSerializer
        return GroupDetailSerializer if self.action == "retrieve" else GroupListSerializer

    def get_queryset(self):
        return Group.objects.all()

    def perform_create(self, serializer):
        group = serializer.save()
        GroupMember.objects.create(group=group, user=self.request.user, role=GroupMember.Role.OWNER)

    def _ensure_group_role(self, group, min_role):
        if self.request.user.is_superuser:
            return
        if not GroupMember.objects.filter(
            group=group,
            user=self.request.user,
            role__gte=min_role,
        ).exists():
            raise PermissionDenied("You do not have permission to manage this group.")

    def perform_update(self, serializer):
        self._ensure_group_role(self.get_object(), GroupMember.Role.OWNER)
        serializer.save()

    def perform_destroy(self, instance):
        self._ensure_group_role(instance, GroupMember.Role.OWNER)
        instance.delete()

    @action(methods=["get", "post"], detail=True)
    def members(self, request, id=None):
        group = self.get_object()

        if request.method == "GET":
            members = GroupMember.objects.filter(group=group)
            return Response(
                GroupMemberSerializer(
                    members, many=True, context={"request": request}
                ).data
            )

        self._ensure_group_role(group, GroupMember.Role.MAINTAINER)
        serializer = GroupMemberSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        member = serializer.save(group=group)
        return Response(
            GroupMemberSerializer(member, context={"request": request}).data,
            status=status.HTTP_201_CREATED,
        )

    @action(methods=["delete"], detail=True, url_path="members/(?P<member_id>[^/.]+)")
    def remove_member(self, request, id=None, member_id=None):
        group = self.get_object()
        self._ensure_group_role(group, GroupMember.Role.MAINTAINER)
        member = get_object_or_404(GroupMember, id=member_id, group=group)
        member.delete()
        return Response(status=status.HTTP_204_NO_CONTENT)

    @action(methods=["get"], detail=True)
    def projects(self, request, id=None):
        group = self.get_object()
        repos = Repository.objects.filter(owner_type="organization", owner_id=group.id)
        is_group_member = GroupMember.objects.filter(group=group, user=request.user).exists()
        if not is_group_member and not request.user.is_superuser:
            repos = repos.filter(
                Q(visibility=Repository.Visibility.PUBLIC) | Q(access_list__user=request.user)
            ).distinct()
        from apps.api.views.projects import RepositorySerializer

        page = self.paginate_queryset(repos)
        if page is not None:
            return self.get_paginated_response(RepositorySerializer(page, many=True).data)
        return Response(RepositorySerializer(repos, many=True).data)

    @action(methods=["get", "post"], detail=True)
    def teams(self, request, id=None):
        group = self.get_object()

        if request.method == "GET":
            teams = Team.objects.filter(group=group).prefetch_related("members")
            return Response(TeamSerializer(teams, many=True).data)

        self._ensure_group_role(group, GroupMember.Role.MAINTAINER)
        serializer = TeamSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        team = serializer.save(group=group)
        return Response(TeamSerializer(team).data, status=status.HTTP_201_CREATED)

    @action(methods=["get"], detail=True, url_path="teams/(?P<team_id>[^/.]+)/members")
    def team_members(self, request, id=None, team_id=None):
        group = self.get_object()
        team = get_object_or_404(Team, id=team_id, group=group)
        memberships = TeamMembership.objects.filter(team=team)
        return Response(TeamMembershipSerializer(memberships, many=True).data)

    @action(methods=["post"], detail=True, url_path="teams/(?P<team_id>[^/.]+)/members")
    def add_team_member(self, request, id=None, team_id=None):
        group = self.get_object()
        self._ensure_group_role(group, GroupMember.Role.MAINTAINER)
        team = get_object_or_404(Team, id=team_id, group=group)
        serializer = TeamMembershipSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        membership = serializer.save(team=team)
        return Response(
            TeamMembershipSerializer(membership).data,
            status=status.HTTP_201_CREATED,
        )
