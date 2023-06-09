import logging
import httpx
from http import HTTPStatus
from openpyxl import load_workbook
from tempfile import NamedTemporaryFile
import lxml.html
import re
from fastapi.concurrency import run_in_threadpool
from fastapi import HTTPException
from random import shuffle
from bson.json_util import dumps
import tarfile
import os
import asyncio
from pycountry import countries
import base64

from core.database import PyObjectId, db
from core.config import settings
from models.pilots import Pilot
from models.teams import Team
from models.judges import Judge
from models.tricks import Trick
from models.competitions import Competition
from models.svg_data import SvgData
from controllers.pilots import PilotCtrl

log = logging.getLogger(__name__)

class UtilsCtrl:
    @staticmethod
    async def backup():
        ret = None
        with NamedTemporaryFile(suffix=".tar.gz", delete=False) as f:
            ret = f.name
            with tarfile.open(mode='w:gz', fileobj=f) as tar:
                for collection in await db.list_collection_names():
                    objects = []
                    for o in await db[collection].find().to_list(1000):
                        objects.append(o)
                    with NamedTemporaryFile(suffix=".json") as json:
                        json.write(dumps(objects).encode())
                        json.flush() # ensure everything is written to disk before calling tar.add
                        tar.add(json.name, arcname=f"{db.name}/{collection}.json")
        
        return ret

    @staticmethod
    async def delete_unused_pilots() -> bool:
        for pilot in await PilotCtrl.get_all_unused_pilots():
            log.info(f"deleting pilot {pilot}")
            await Pilot.delete(pilot)

    @staticmethod
    def svg(datas: list[SvgData]):
        line_sep = 40
        font_size = 13
        text_color = '#696969'
        width = 500
        height = line_sep * (1 + len(datas))

        svg = []
        svg.append('<?xml version="1.0" encoding="UTF-8"?>')
        svg.append(f'<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{width}" height="{height}" viewBox="0 0 {width} {height}" style="border-radius:20px;">')


        # Headers
        svg.append(f'<rect x="0" y="0" width="{width}" height="{line_sep}" fill="#353535" fill-opacity="0.4" />')
        svg.append(f'    <text x="20" y="{0.6*line_sep}" font-size="{font_size}" font-family="Exo" font-style="italic" font-weight="700" fill="#fafafa">Rank</text>')
        svg.append(f'    <text x="90" y="{0.6*line_sep}" font-size="{font_size}" font-family="Exo" font-style="italic" font-weight="700" fill="#fafafa">Pilot</text>')
        svg.append(f'    <text x="480" y="{0.6*line_sep}" font-size="{font_size}" font-family="Exo" font-style="italic" font-weight="700" fill="#fafafa" text-anchor="end">Score</text>')

        for i, data in enumerate(datas):
            country = countries.get(alpha_3=data.country).alpha_2.lower()
            flag = base64.b64encode(open(f"flags/{country}.svg", "rb").read()).decode('ascii')
            svg.append(f'<rect x="0" y="{1*line_sep + line_sep*i}" width="{width}" height="{line_sep}" fill="{"#e5e5e5" if i%2==0 else "#f5f5f5"}" fill-opacity="0.4" />')
            svg.append(f'    <text x="20" y="{1.6*line_sep + line_sep*i}" font-size="{font_size}" font-family="Exo" font-style="italic" font-weight="700" fill="#696969">{data.rank}</text>')
            svg.append(f'    <image fill="#000000" x="60" y="{1.6*line_sep + line_sep*i - 15}" width="20"  href="data:image/svg+xml;base64,{flag}"/>')
            svg.append(f'    <text x="90" y="{1.6*line_sep + line_sep*i}" font-size="{font_size}" font-family="Exo" font-style="normal" font-weight="700" fill="#000000">{data.name}</text>')
            svg.append(f'    <text x="480" y="{1.6*line_sep + line_sep*i}" font-size="{font_size}" font-family="Exo" font-style="normal" font-weight="700" fill="#000000" text-anchor="end">{data.score}</text>')

#        svg.append(f'<rect x="0" y="0" width="{width}" height="{height}" fill="none" stroke="black"/>')

        svg.append('</svg>')
        svg = "\n".join(svg)

        return svg
