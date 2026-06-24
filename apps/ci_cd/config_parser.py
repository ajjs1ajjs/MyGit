import yaml


class CiConfigError(Exception):
    pass


def parse_ci_config(content: str) -> dict:
    try:
        config = yaml.safe_load(content)
    except yaml.YAMLError as e:
        raise CiConfigError(f"Invalid YAML: {e}") from e

    if not isinstance(config, dict):
        raise CiConfigError("Config must be a YAML dictionary.")

    stages = config.get("stages", [])
    jobs = {}

    for key, value in config.items():
        if key == "stages":
            continue
        if not isinstance(value, dict):
            continue
        job = {
            "name": key,
            "stage": value.get("stage", "test"),
            "image": value.get("image", "python:3.12-slim"),
            "script": value.get("script", []),
            "artifacts": value.get("artifacts", {}),
            "only": value.get("only", []),
            "except": value.get("except", []),
        }
        jobs[key] = job

    return {
        "stages": stages or list(dict.fromkeys(j["stage"] for j in jobs.values())),
        "jobs": jobs,
    }


def build_default_config() -> dict:
    return {
        "stages": ["test"],
        "jobs": {
            "test": {
                "name": "test",
                "stage": "test",
                "image": "python:3.12-slim",
                "script": ["pip install -r requirements.txt", "pytest"],
                "artifacts": {},
                "only": [],
                "except": [],
            }
        },
    }
