#!/usr/bin/env -S uv run --script
#
# /// script
# dependencies = [
#   "aiohttp",
# ]
# ///

import asyncio
from collections import defaultdict
from collections.abc import AsyncIterator
import dataclasses
from itertools import chain
import json
from pathlib import Path
from tempfile import TemporaryDirectory
from typing import Self

import aiohttp

user = "kwohlfahrt"
repo = "tf-k8s"


class Gpg:
    def __init__(self, key_name: str, dir: Path):
        self.key_name = key_name
        self.dir = dir
        self._armor = None
        self._key_id = None

    async def armor(self) -> str:
        if self._armor is not None:
            return self._armor

        proc = await asyncio.create_subprocess_exec(
            "gpg",
            f"--homedir={self.dir}",
            "--armor",
            "--export",
            self.key_name,
            stdout=asyncio.subprocess.PIPE,
        )
        stdout, _ = await proc.communicate()
        assert proc.returncode == 0

        self._armor = stdout.decode()
        return self._armor

    async def key_id(self) -> str:
        if self._key_id is not None:
            return self._key_id

        proc = await asyncio.create_subprocess_exec(
            "gpg",
            f"--homedir={self.dir}",
            "--keyid-format=long",
            "--list-keys",
            "--with-colons",
            self.key_name,
            stdout=asyncio.subprocess.PIPE,
        )
        stdout, _ = await proc.communicate()
        assert proc.returncode == 0
        for line in stdout.decode().splitlines():
            parts = line.split(":")
            if parts[0] == "pub":
                self._key_id = parts[4]
                return self._key_id

    async def sign(self, file: Path):
        proc = await asyncio.create_subprocess_exec(
            "gpg",
            f"--homedir={self.dir}",
            "--batch",
            "--passphrase=",
            "--local-user",
            self.key_name,
            "--detach-sign",
            str(file),
        )
        await proc.wait()
        assert proc.returncode == 0

    @classmethod
    async def new(cls, key_name: str, dir: Path):
        proc = await asyncio.create_subprocess_exec(
            "gpg",
            f"--homedir={dir}",
            "--batch",
            "--passphrase=",
            "--quick-generate-key",
            key_name,
            "ed25519",
            "sign",
            "never",
        )
        await proc.wait()
        assert proc.returncode == 0
        return cls(key_name, dir)


@dataclasses.dataclass
class Platform:
    os: str
    arch: str


@dataclasses.dataclass
class Release:
    provider: str
    version: str
    protocols: list[str]
    platforms: list[Platform]
    assets: dict[str, str]
    shasums: str

    @property
    def version_doc(self):
        return {
            "version": self.version,
            "protocols": self.protocols,
            "platforms": list(map(dataclasses.asdict, self.platforms)),
        }

    async def write_files(self, base: Path, gpg: Gpg):
        download = Path(self.provider) / self.version / "download"
        (base / download).mkdir(exist_ok=True, parents=True)

        with TemporaryDirectory() as d:
            shasums = Path(d) / "SHA256SUMS"
            shasums.write_text(self.shasums)
            await gpg.sign(shasums)
            shasums.with_suffix(".sig").rename(base / download / "SHA256SUMS.sig")

        shasums_url = f"https://{user}.github.io/{repo}/registry/providers/v1/{repo}/{download}/SHA256SUMS.sig"
        file_hashes = {
            filename: shasum
            for shasum, filename in map(str.split, self.shasums.splitlines())
        }

        for platform in self.platforms:
            path = base / download / platform.os / platform.arch
            path.parent.mkdir(parents=True, exist_ok=True)
            filename = f"terraform-provider-{self.provider}_{self.version}_{platform.os}_{platform.arch}.zip"
            path.write_text(
                json.dumps(
                    {
                        "protocols": self.protocols,
                        "os": platform.os,
                        "arch": platform.arch,
                        "filename": filename,
                        "download_url": self.assets[filename],
                        "shasums_url": self.assets["SHA256SUMS"],
                        "shasums_signature_url": shasums_url,
                        "shasum": file_hashes[filename],
                        "signing_keys": {
                            "gpg_public_keys": [
                                {
                                    "key_id": await gpg.key_id(),
                                    "ascii_armor": await gpg.armor(),
                                }
                            ]
                        },
                    }
                )
            )

    @classmethod
    async def from_json(cls, sess: aiohttp.ClientSession, json) -> Self:
        provider, _ = json["name"].split(maxsplit=1)
        assets = {a["name"]: a["browser_download_url"] for a in json["assets"]}
        async with sess.get(assets["version.json"]) as r:
            r.raise_for_status()
            version = await r.json(content_type="application/octet-stream")

        async with sess.get(assets["SHA256SUMS"]) as r:
            r.raise_for_status()
            shasums = await r.text()

        return cls(
            provider="k8s-" + provider,
            version=version["version"],
            protocols=version["protocols"],
            platforms=[Platform(**p) for p in version["platforms"]],
            assets=assets,
            shasums=shasums,
        )


async def get_releases(sess: aiohttp.ClientSession) -> AsyncIterator[Release]:
    headers = {
        "Accept": "application/vnd.github+json",
        "X-GitHub-Api-Version": "2022-11-28",
    }

    page = f"https://api.github.com/repos/{user}/{repo}/releases"
    while page:
        async with sess.get(page, headers=headers) as r:
            r.raise_for_status()
            page = r.links.get("next", {}).get("url", None)
            for item in await r.json():
                if any(a["name"] == "version.json" for a in item["assets"]):
                    yield await Release.from_json(sess, item)


async def main():
    out_path = Path(".")
    with TemporaryDirectory() as d:
        key = await Gpg.new("GitHub Actions Bot", dir=d)
        releases = defaultdict(list)

        async with aiohttp.ClientSession() as sess:
            async for r in get_releases(sess):
                releases[r.provider].append(r)

        base_path = out_path / "registry" / "providers" / "v1" / repo
        for provider, provider_releases in releases.items():
            path = base_path / provider / "versions"
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_text(
                json.dumps({"versions": [r.version_doc for r in provider_releases]})
            )

        await asyncio.gather(
            *(
                r.write_files(base_path, key)
                for r in chain.from_iterable(releases.values())
            )
        )


if __name__ == "__main__":
    asyncio.run(main())
