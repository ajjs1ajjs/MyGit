from django.db import migrations


def seed_default_admin(apps, schema_editor):
    User = apps.get_model("accounts", "User")
    admin, created = User.objects.get_or_create(
        username="admin",
        defaults={
            "email": "admin@users.mygit.local",
            "is_active": True,
        },
    )
    admin.email = admin.email or "admin@users.mygit.local"
    admin.is_superuser = True
    admin.is_staff = True
    admin.is_active = True
    admin.must_change_password = True
    update_fields = ["email", "is_superuser", "is_staff", "is_active", "must_change_password"]
    admin.save(update_fields=update_fields)


class Migration(migrations.Migration):
    dependencies = [
        ("accounts", "0002_user_must_change_password"),
    ]

    operations = [
        migrations.RunPython(seed_default_admin, migrations.RunPython.noop),
    ]
