import logging
import itertools
from models.tricks import Trick, Bonus, UniqueTrick
from core.config import settings

log = logging.getLogger(__name__)

class TrickCtrl:
    @staticmethod
    def start():
        Trick.createIndexes()

    @staticmethod
    def generate_tricks(trick: Trick):
        combinations = []
        for i in range(1,len(trick.bonuses)+1):
            for combination in  list(itertools.combinations(trick.bonuses, i)):
                ignore = False
                for pair in itertools.combinations(combination, min(i, 2)):
                    pair = list(map(lambda x: x.name, list(pair)))
                    pair.sort()
                    for constraint in settings.tricks.bonus_constraints:
                        constraint.sort()
                        if pair == constraint:
                            ignore = True
                            continue
                    if ignore:
                        continue

                if not ignore:
                    combinations.append(list(combination))

        # cleanup tricks before generating
        trick.tricks = []

        # add standard trick (without bonus)
        TrickCtrl.generate_trick(trick, [])

        for combination in combinations:
            TrickCtrl.generate_trick(trick, combination)

    @staticmethod
    def generate_trick(trick: Trick, combination: [Bonus]):

        pre_name = ""
        pre_acronym = ""
        post_name = ""
        post_acronym = ""
        overall_bonus = 0.0
        bonus_types = []

        for bonus in combination:
            b = next(d for d in settings.tricks.available_bonuses if d['name'] == bonus.name)
            overall_bonus += bonus.bonus
            pre = b.get('pre_acronym')
            post = b.get('post_acronym')
            if pre:
                pre_acronym = f"{pre_acronym}{pre}"
                if pre_name:
                    pre_name = f"{pre_name} {bonus.name}"
                else:
                    pre_name = bonus.name
            if post:
                post_acronym = f"{post_acronym}{post}"
                if post_name:
                    post_name = f"{post_name} {bonus.name}"
                else:
                    post_name = bonus.name

            if b.get('type') not in bonus_types:
                bonus_types.append(b.get('type'))

        name = f"%s{trick.name}"
        if pre_name:
            name = f"{pre_name} {name}"
        if post_name:
            name = f"{name} {post_name}"

        acronym = f"%s{trick.acronym}"
        if pre_acronym:
            acronym = f"{pre_acronym}{acronym}"
        if post_acronym:
            acronym = f"{acronym}{post_acronym}"

        uniqueness = []
        if "reverse" in bonus_types:
            uniqueness.append("reverse")

        if len(trick.directions) == 0:
            trick.tricks.append(UniqueTrick(
                name = (name % ""),
                acronym = (acronym % ""),
                technical_coefficient = trick.technical_coefficient,
                bonus = overall_bonus,
                bonus_types = bonus_types,
                base_trick = trick.name,
                uniqueness = uniqueness,
            ))
        else:
            for direction in trick.directions:
                direction_acronym = next(d['acronym'] for d in settings.tricks.available_directions if d['name'] == direction)

                trick.tricks.append(UniqueTrick(
                    name = (name % f"{direction} "),
                    acronym = (acronym % direction_acronym),
                    technical_coefficient = trick.technical_coefficient,
                    bonus = overall_bonus,
                    bonus_types = bonus_types,
                    base_trick = trick.name,
                    uniqueness = uniqueness + [direction],
                ))

        return

    @staticmethod
    async def check_tricks_unicity():
        tricks = await Trick.get_unique_tricks(solo=True, synchro=True)
        acronyms  = list(map(lambda x: x.acronym, tricks))
        duplicates = [a for a in acronyms if acronyms.count(a) > 1 ]
        if len(duplicates) > 0:
            raise HTTPException(status_code=400, detail="The following acronyms are defined twice: %s" % ",".join(duplicates))
        return True

        all_acronyms  = list(map(lambda x: x.acronym, tricks))
        all_acronyms.sort()
        unique_acronyms = list(set(all_acronyms))
        return len(all_acronyms) == len(unique_acronyms)


