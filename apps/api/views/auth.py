from django.conf import settings
from django.contrib.auth import get_user_model
from django.contrib.auth.password_validation import validate_password
from django.contrib.auth.tokens import PasswordResetTokenGenerator
from django.core.exceptions import ValidationError as DjangoValidationError
from django.core.mail import send_mail
from django.template.loader import render_to_string
from django.utils import timezone
from django.utils.decorators import method_decorator
from django.views.decorators.csrf import csrf_exempt
from rest_framework import serializers, status, viewsets
from rest_framework.decorators import action
from rest_framework.permissions import AllowAny
from rest_framework.response import Response
from rest_framework_simplejwt.tokens import RefreshToken

from apps.accounts.ldap_auth import authenticate_ldap_user

User = get_user_model()
token_generator = PasswordResetTokenGenerator()


class RegisterSerializer(serializers.Serializer):
    username = serializers.CharField(max_length=150)
    email = serializers.EmailField(required=False, allow_blank=True)
    password = serializers.CharField(min_length=8, write_only=True)
    full_name = serializers.CharField(max_length=255, required=False, allow_blank=True)

    def validate_username(self, value):
        if User.objects.filter(username=value).exists():
            raise serializers.ValidationError("Username already taken.")
        return value

    def validate_email(self, value):
        if not value:
            return value
        if User.objects.filter(email=value).exists():
            raise serializers.ValidationError("Email already registered.")
        return value

    def validate_password(self, value):
        try:
            validate_password(value)
        except DjangoValidationError as e:
            raise serializers.ValidationError(list(e.messages)) from e
        return value

    def create(self, validated_data):
        user = User.objects.create_user(
            username=validated_data["username"],
            email=validated_data.get("email", ""),
            password=validated_data["password"],
            full_name=validated_data.get("full_name", ""),
        )
        return user


class LoginSerializer(serializers.Serializer):
    login = serializers.CharField(max_length=255)
    password = serializers.CharField(write_only=True)


class AuthViewSet(viewsets.GenericViewSet):
    permission_classes = [AllowAny]

    @method_decorator(csrf_exempt)
    def dispatch(self, *args, **kwargs):
        return super().dispatch(*args, **kwargs)

    @action(methods=["post"], detail=False)
    def register(self, request):
        serializer = RegisterSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        user = serializer.save()
        refresh = RefreshToken.for_user(user)
        return Response(
            {
                "user": {"id": str(user.id), "username": user.username, "email": user.email},
                "access": str(refresh.access_token),
                "refresh": str(refresh),
            },
            status=status.HTTP_201_CREATED,
        )

    @action(methods=["post"], detail=False)
    def login(self, request):
        serializer = LoginSerializer(data=request.data)
        serializer.is_valid(raise_exception=True)
        login_id = serializer.validated_data["login"]
        password = serializer.validated_data["password"]

        user = User.objects.filter(username=login_id).first()
        if not user and "@" in login_id:
            user = User.objects.filter(email=login_id).first()

        local_password_ok = bool(user and user.check_password(password))
        if not local_password_ok:
            user = authenticate_ldap_user(login_id, password)

        if not user:
            return Response({"detail": "Invalid credentials."}, status=status.HTTP_401_UNAUTHORIZED)

        if not user.is_active:
            return Response({"detail": "Account is disabled."}, status=status.HTTP_401_UNAUTHORIZED)

        refresh = RefreshToken.for_user(user)
        return Response(
            {
                "user": {
                    "id": str(user.id),
                    "username": user.username,
                    "email": user.email,
                    "must_change_password": user.must_change_password,
                },
                "access": str(refresh.access_token),
                "refresh": str(refresh),
            }
        )

    @action(methods=["post"], detail=False)
    def refresh(self, request):
        refresh_token = request.data.get("refresh")
        if not refresh_token:
            return Response(
                {"detail": "Refresh token required."}, status=status.HTTP_400_BAD_REQUEST
            )
        try:
            refresh = RefreshToken(refresh_token)
            return Response({"access": str(refresh.access_token)})
        except Exception:
            return Response(
                {"detail": "Invalid or expired token."}, status=status.HTTP_401_UNAUTHORIZED
            )

    @action(methods=["post"], detail=False, url_path="password-reset")
    def password_reset(self, request):
        email = request.data.get("email", "")
        user = User.objects.filter(email=email, is_active=True).first()

        if user:
            import threading

            thread = threading.Thread(target=self._send_reset_email, args=(user,))
            thread.start()

        return Response(
            {"detail": "If this email exists, a reset link has been sent."},
            status=status.HTTP_200_OK,
        )

    def _send_reset_email(self, user):
        token = token_generator.make_token(user)
        site_name = getattr(settings, "MYGIT_SITE_NAME", "MyGit")
        context = {
            "user": user,
            "token": token,
            "site_name": site_name,
        }
        subject = render_to_string("email/password_reset_subject.txt", context)
        body = render_to_string("email/password_reset_body.txt", context)

        send_mail(
            subject=subject.strip(),
            message=body,
            from_email=None,
            recipient_list=[user.email],
            fail_silently=True,
        )

        return Response(
            {"detail": "If this email exists, a reset link has been sent."},
            status=status.HTTP_200_OK,
        )

    @action(methods=["post"], detail=False, url_path="password-reset/confirm")
    def password_reset_confirm(self, request):
        email = request.data.get("email", "")
        token = request.data.get("token", "")
        password = request.data.get("password", "")

        if not password or len(password) < 8:
            return Response(
                {"detail": "Password must be at least 8 characters."},
                status=status.HTTP_400_BAD_REQUEST,
            )

        try:
            user = User.objects.get(email=email, is_active=True)
        except User.DoesNotExist:
            return Response(
                {"detail": "Invalid reset link."},
                status=status.HTTP_400_BAD_REQUEST,
            )

        if not token_generator.check_token(user, token):
            return Response(
                {"detail": "Invalid or expired reset token."},
                status=status.HTTP_400_BAD_REQUEST,
            )

        try:
            validate_password(password, user)
        except DjangoValidationError as e:
            return Response({"detail": list(e.messages)}, status=status.HTTP_400_BAD_REQUEST)

        user.set_password(password)
        user.last_login = timezone.now()
        user.save(update_fields=["password", "last_login"])

        return Response(
            {"detail": "Password has been reset successfully."},
            status=status.HTTP_200_OK,
        )

    @action(methods=["post"], detail=False)
    def logout(self, request):
        refresh_token = request.data.get("refresh")
        if refresh_token:
            try:
                token = RefreshToken(refresh_token)
                token.blacklist()
            except Exception:
                pass
        return Response(status=status.HTTP_204_NO_CONTENT)
