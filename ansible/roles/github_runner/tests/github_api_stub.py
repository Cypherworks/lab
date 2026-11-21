"""A stub of GitHub's org runner-group REST endpoints for runner_group.yml.

Serves the five calls the role makes, with the status codes and body fields
GitHub documents (docs.github.com/en/rest/actions/self-hosted-runner-groups),
keeps the groups in a JSON state file the test can seed, and appends every
request (method, path, body) to a JSON-lines log the test asserts on. Any
token but the expected one gets a 401, as GitHub answers a bad token.

    python3 -I github_api_stub.py <port> <token> <state.json> <requests.log>
"""
import json
import re
import sys
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT, TOKEN, STATE, LOG = int(sys.argv[1]), sys.argv[2], sys.argv[3], sys.argv[4]
GROUPS = re.compile(r"^/orgs/[^/]+/actions/runner-groups$")
GROUP = re.compile(r"^/orgs/[^/]+/actions/runner-groups/(\d+)$")
REPOS = re.compile(r"^/orgs/[^/]+/actions/runner-groups/(\d+)/repositories$")


def load():
    with open(STATE) as fh:
        return json.load(fh)


def save(groups):
    with open(STATE, "w") as fh:
        json.dump(groups, fh)


def public(group):
    return {k: v for k, v in group.items() if k != "repository_ids"}


class Handler(BaseHTTPRequestHandler):
    def log_message(self, *args):
        pass

    def reply(self, status, body=None):
        data = b"" if body is None else json.dumps(body).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def handle_call(self):
        length = int(self.headers.get("Content-Length") or 0)
        body = json.loads(self.rfile.read(length)) if length else None
        path = self.path.split("?")[0]
        with open(LOG, "a") as fh:
            fh.write(json.dumps({"method": self.command, "path": path, "body": body}) + "\n")
        if self.headers.get("Authorization") != "Bearer " + TOKEN:
            return self.reply(401, {"message": "Bad credentials"})
        groups = load()
        if GROUPS.match(path) and self.command == "GET":
            return self.reply(200, {"total_count": len(groups),
                                    "runner_groups": [public(g) for g in groups]})
        if GROUPS.match(path) and self.command == "POST":
            group = {"id": 1 + max([g["id"] for g in groups] or [0]),
                     "name": body["name"], "visibility": body.get("visibility", "all"),
                     "default": False, "inherited": False,
                     "allows_public_repositories": body.get("allows_public_repositories", False),
                     "repository_ids": body.get("selected_repository_ids", [])}
            save(groups + [group])
            return self.reply(201, public(group))
        match = GROUP.match(path) or REPOS.match(path)
        found = [g for g in groups if match and g["id"] == int(match.group(1))]
        if not found:
            return self.reply(404, {"message": "Not Found"})
        group = found[0]
        if GROUP.match(path) and self.command == "PATCH":
            group.update({k: body[k] for k in ("name", "visibility", "allows_public_repositories") if k in body})
            save(groups)
            return self.reply(200, public(group))
        if REPOS.match(path) and self.command == "GET":
            repos = [{"id": i, "name": "repo-%d" % i} for i in group["repository_ids"]]
            return self.reply(200, {"total_count": len(repos), "repositories": repos})
        if REPOS.match(path) and self.command == "PUT":
            group["repository_ids"] = body["selected_repository_ids"]
            save(groups)
            return self.reply(204)
        return self.reply(404, {"message": "Not Found"})

    do_GET = do_POST = do_PATCH = do_PUT = handle_call


if __name__ == "__main__":
    HTTPServer(("127.0.0.1", PORT), Handler).serve_forever()
