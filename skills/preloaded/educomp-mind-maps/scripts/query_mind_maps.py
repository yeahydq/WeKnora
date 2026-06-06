#!/usr/bin/env python3
from __future__ import annotations

import json
import os
import sys
import urllib.error
import urllib.parse
import urllib.request


def load_payload() -> dict:
    raw = sys.stdin.read().strip()
    if not raw:
        return {}
    data = json.loads(raw)
    if not isinstance(data, dict):
        raise ValueError("input must be a JSON object")
    return data


def load_endpoints() -> list[dict]:
    raw = os.getenv("EDUCOMP_ENDPOINTS_JSON", "").strip()
    endpoints: list[dict] = []
    if raw:
        try:
            parsed = json.loads(raw)
            if isinstance(parsed, list):
                for item in parsed:
                    if not isinstance(item, dict):
                        continue
                    endpoint_id = str(item.get("id", "")).strip()
                    base_url = str(item.get("base_url", "")).strip().rstrip("/")
                    api_key = str(item.get("api_key", "")).strip()
                    if endpoint_id and base_url and api_key:
                        endpoints.append({"id": endpoint_id, "name": str(item.get("name", endpoint_id)).strip() or endpoint_id, "base_url": base_url, "api_key": api_key})
        except json.JSONDecodeError:
            pass
    if endpoints:
        return endpoints
    base_url = os.getenv("EDUCOMP_BASE_URL", "").strip().rstrip("/")
    api_key = os.getenv("EDUCOMP_API_KEY", "").strip()
    if base_url and api_key:
        return [{"id": "default", "name": "EduComp", "base_url": base_url, "api_key": api_key}]
    return []


def resolve_endpoint(payload: dict, endpoints: list[dict]) -> dict:
    endpoint_id = str(payload.get("endpoint_id") or os.getenv("WEKNORA_EDUCOMP_DEFAULT_ENDPOINT_ID") or "").strip()
    if endpoint_id:
        for item in endpoints:
            if item["id"] == endpoint_id:
                return item
    if endpoints:
        return endpoints[0]
    raise RuntimeError("EduComp endpoints are not configured")


def build_url(endpoint: dict, payload: dict) -> str:
    mode = str(payload.get("mode") or "list").strip()
    query = {}
    if payload.get("q"):
        query["q"] = str(payload["q"])
    if payload.get("subject"):
        query["subject"] = str(payload["subject"])
    if payload.get("user_id"):
        query["userId"] = str(payload["user_id"])
    if payload.get("has_wrong_questions") is not None:
        query["hasWrongQuestions"] = "true" if bool(payload.get("has_wrong_questions")) else "false"
    query["skip"] = str(payload.get("skip", 0))
    query["take"] = str(payload.get("take", 20))
    path = "/api/mind-maps"
    if mode == "knowledge_points_list":
        path = "/api/mind-maps/knowledge-points/list"
    return endpoint["base_url"] + path + "?" + urllib.parse.urlencode(query)


def main() -> int:
    try:
        payload = load_payload()
        endpoint = resolve_endpoint(payload, load_endpoints())
        url = build_url(endpoint, payload)
        request = urllib.request.Request(url, headers={"X-API-Key": endpoint["api_key"], "Accept": "application/json"})
        with urllib.request.urlopen(request, timeout=12) as response:
            body = response.read().decode("utf-8")
        data = json.loads(body)
        print(json.dumps({
            "success": True,
            "endpoint": {"id": endpoint["id"], "name": endpoint["name"], "base_url": endpoint["base_url"]},
            "mode": payload.get("mode", "list"),
            "result": data,
        }, ensure_ascii=False, indent=2))
        return 0
    except urllib.error.HTTPError as exc:
        detail = exc.read().decode("utf-8", errors="replace")
        print(json.dumps({"success": False, "error": f"HTTP {exc.code}", "detail": detail}, ensure_ascii=False, indent=2))
        return 1
    except urllib.error.URLError as exc:
        print(json.dumps({"success": False, "error": "EduComp unreachable", "detail": str(exc.reason)}, ensure_ascii=False, indent=2))
        return 1
    except Exception as exc:
        print(json.dumps({"success": False, "error": str(exc)}, ensure_ascii=False, indent=2))
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
