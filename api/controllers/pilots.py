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
from core.database import PyObjectId

from core.config import settings
from models.pilots import Pilot
from models.competitions import Competition
from models.judges import Judge
from models.teams import Team

log = logging.getLogger(__name__)

class PilotCtrl:
    @staticmethod
    def start():
        Pilot.createIndexes()

    @staticmethod
    def fetch_and_load_pilots_list(body: bytes):
        with NamedTemporaryFile(suffix=".xlsx") as f:
            f.write(body)
            xls = load_workbook(f.name)
        return xls

    @staticmethod
    async def update_rankings():
        async with httpx.AsyncClient() as client:
            ret = await client.get(settings.pilots.civl_link_all_pilots)

        if ret.status_code != HTTPStatus.OK:
            log.error("unable to update pilots from %s, code=%d", settings.pilots.civl_link_all_pilots, res.status_code)
            return

        html = lxml.html.fromstring(ret.text)

        xls_url = html.cssselect('a.btn-download')[0].get('href')

        # fetch the excel
        async with httpx.AsyncClient() as client:
            ret = await client.get(xls_url)

        if ret.status_code != HTTPStatus.OK:
            log.error("unable to update pilots from %s, code=%d", settings.pilots.civl_link_all_pilots, res.status_code)
            return

        # write the excel to a temporary file and read it
        xls = await run_in_threadpool(lambda: PilotCtrl.fetch_and_load_pilots_list(ret.content))
        sheet = xls.active # get the first and only sheet
        # loop over each cells of column B (where the CIVL id are)
        # and extract civlids
        for cell in sheet['B']:
            try:
                # convert the cell content to int
                # if conversion can be made we assume it's a CIVLID
                civlid = int(cell.value or '')
                pilot = await Pilot.get(civlid)
                pilot.rank = int(cell.offset(column=-1).value or '')
                await pilot.save()
            except ValueError:
                continue

    @staticmethod
    async def update_pilots():
        # fetch the excel
        async with httpx.AsyncClient() as client:
            ret = await client.get(settings.pilots.civl_link_all_pilots)

        if ret.status_code != HTTPStatus.OK:
            log.error("unable to update pilots from %s, code=%d", settings.pilots.civl_link_all_pilots, res.status_code)
            return

        # write the excel to a temporary file and read it
        xls = await run_in_threadpool(lambda: PilotCtrl.fetch_and_load_pilots_list(ret.content))
        sheet = xls.active # get the first and only sheet
        # loop over each cells of column B (where the CIVL id are)
        # and extract civlids
        civlids = []
        for cell in sheet['B']:
            try:
                # convert the cell content to int
                # if conversion can be made we assume it's a CIVLID
                civlid = int(cell.value or '')
                civlids.append(civlid)
            except:
                continue
        # loop over each civlids found and create or update pilot
        shuffle(civlids)
        for civlid in civlids:
            try:
                await PilotCtrl.update_pilot(civlid)
            except Exception as e:
                log.exception("Exception while updating pilot with CIVL ID #%d", civlid)

    @staticmethod
    async def update_pilot(civlid: int):
        log.debug(f"Updating pilot #{civlid}")
        async with httpx.AsyncClient() as client:
            link = settings.pilots.civl_link_one_pilot + str(civlid)

            try:
                ret = await client.get(link)
            except httpx.HTTPError as exc:
                log.error(f"Connection failed to CIVL website ({link})", exc)
                raise HTTPException(status_code=500, detail=f"Connection failed to CIVL website")

            if ret.status_code == HTTPStatus.NOT_FOUND:
                raise HTTPException(status_code=404, detail=f"Pilot not found in CIVL database")
            if ret.status_code != HTTPStatus.OK:
                raise HTTPException(status_code=500, detail=f"Problem while fetch pilot information from CIVL database")

        html = lxml.html.fromstring(ret.text)

        name = html.cssselect('h1.title-pilot')[0].text_content()

        i = html.cssselect('div.country-place i')[0]
        i.classes.discard('flag')
        country = re.sub(r'^flag-', '', i.get('class'))

        about = ''
        for e in html.cssselect('article#info p'):
            about += ''.join(e.itertext()).replace('\n', ' ')

        links = []
        for e in html.cssselect('a.social-link'):
            e.classes.discard('social-link')
            links.append({
                "name":e.get('class'),
                "link":e.get('href')
            })

        for e in html.cssselect('article.links-block a'):
            text = ''.join(e.itertext()).replace('\n', ' ')
            links.append({
                "name":text,
                "link":e.get('href')
            })

        sponsors = []
        for e in html.cssselect('aside.sponsors-wrapper a'):
            sponsors.append({
                "name": e.get('alt'),
                "link": e.get('href'),
                "img": e.cssselect('img')[0].get('src'),
            })

        photo = html.cssselect('.photo-pilot img')[0].get('src')

        background_picture = html.cssselect('.image-fon img')[0].get('src')

        async with httpx.AsyncClient() as client:
            link = settings.pilots.civl_link_one_pilot + str(civlid) + '/ranking?discipline_id=5'

            try:
                ret = await client.get(link)
            except httpx.HTTPError as exc:
                raise HTTPException(status_code=500, detail=f"Connection failed to CIVL website  (ranking)")

            if ret.status_code == HTTPStatus.NOT_FOUND:
                raise HTTPException(status_code=404, detail=f"Pilot ranking not found in CIVL database")
            if ret.status_code != HTTPStatus.OK:
                raise HTTPException(status_code=500, detail=f"Problem while fetch pilot ranking from CIVL database")

        rank = 9999
        html = lxml.html.fromstring(ret.text)
        divs = html.cssselect('div#w1 div.col-2:nth-child(3)')
        if divs is not None and len(divs) > 0:
            rank = int(divs[0].text_content())

        pilot = Pilot(
            id=civlid,
            civlid=civlid,
            name=name,
            country=country,
            about=about,
            link=link,
            links=links,
            sponsors=sponsors,
            photo=photo,
            background_picture=background_picture,
            rank = rank,
        )

        # keep gender as CIVL does not permit, yet, do retrieve pilot gender
        try:
            previousPilot = await Pilot.get(civlid)
            pilot.gender = previousPilot.gender
        except Exception:
            pass

        return await pilot.save()

    @staticmethod
    async def get_all_used_pilots():
        pilots = []

        # retrieve pilots from each competition
        for competition in await Competition.getall():
            pilots += competition.pilots

        # also add pilots in each team
        for team  in await Team.getall():
            pilots += team.pilots

        # consider judges with a civlid as a valuable pilot
        for judge in await Judge.getall():
            if judge.civlid is not None:
                pilots.append(judge.civlid)

        # unique
        return list(set(pilots))

    @staticmethod
    async def get_all_unused_pilots():
        used_pilots = await PilotCtrl.get_all_used_pilots()
        all_pilots = await Pilot.getall()

        return [p.civlid for p in all_pilots if p.civlid not in used_pilots]
