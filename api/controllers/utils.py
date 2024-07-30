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
    def svg(datas: list[SvgData], animated: int = -1):
        #datas = datas[0:15]
        max_lines = 10
        line_height = 40
        font_size = 13
        text_color = '#696969'
        width = 500
        height = line_height * (min(max_lines, len(datas)) + 1)
        nb_lines = len(datas)


        svg = []
        svg.append(f'''<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="{width}" height="{height}" viewBox="0 0 {width} {height}" style="border-radius:20px; border-color: black; border-width: 1px;">
''')
        svg.append(f'  <g id="pilots">')
        if nb_lines > max_lines and animated >= 0:
            if len(datas) % 2 == 0:
                datas.append(None) # 2 empty lines when number of lines is even to ensure to keep the colors right
            datas.append(None) # empty line between last and first
            for i in range(10): # add the first 10 at the end to fake a perfect loop
                datas.append(datas[i])

            translate_y = -line_height * (len(datas) - max_lines)
            duration = int(len(datas) - max_lines * 2/3)
            svg.append(f'''
    <animateTransform attributeName="transform" type="translate" values="0 0; 0 {translate_y};" dur="{duration}s" repeatCount="indefinite" begin="{animated}s" />
''')

        for i, data in enumerate(datas):
            if data is None: # handle empty line
                continue

            svg.append(f'''    <rect x="0" y="{line_height * (1 + i)}" width="{width}" height="{line_height}" fill="{"#e5e5e5" if i%2==0 else "#f5f5f5"}" fill-opacity="0.4" />
    <text x="20" y="{line_height * (1 + i + 0.6)}" font-size="{font_size}" font-family="Exo" font-style="italic" font-weight="700" fill="{text_color}">{data.rank}</text>''')
            try:
                country = countries.get(alpha_3=data.country).alpha_2.lower()
                flag = base64.b64encode(open(f"flags/{country}.svg", "rb").read()).decode('ascii')
                svg.append(f'    <image fill="#000000" x="60" y="{line_height * (1 + i + 0.6) - 15}" width="20"  href="data:image/svg+xml;base64,{flag}"/>')
            except:
                pass
            svg.append(f'''    <text x="90" y="{line_height * (1 + i + 0.6)}" font-size="{font_size}" font-family="Exo" font-style="normal" font-weight="700" fill="#000000">{data.name}</text>
    <text x="480" y="{line_height * (1 + i + 0.6)}" font-size="{font_size}" font-family="Exo" font-style="normal" font-weight="700" fill="#000000" text-anchor="end">{data.score:5.3f}</text>
''')


        svg.append(f'  </g> <!-- pilots -->')

        # headers at the end because SVG elements are rendered in order and we want headers to
        # be rendered on top of the results (z-index style management)
        svg.append(f'''
  <g id="headers">
    <rect x="0" y="0" width="{width}" height="{line_height}" fill="#353535" />
    <text x="20" y="{0.6*line_height}" font-size="{font_size}" font-family="Exo" font-style="italic" font-weight="700" fill="#fafafa">Rank</text>
    <text x="90" y="{0.6*line_height}" font-size="{font_size}" font-family="Exo" font-style="italic" font-weight="700" fill="#fafafa">Pilot</text>
     <text x="480" y="{0.6*line_height}" font-size="{font_size}" font-family="Exo" font-style="italic" font-weight="700" fill="#fafafa" text-anchor="end">Score</text>
  </g>
</svg>
''')
        svg = "\n".join(svg)

        return svg
