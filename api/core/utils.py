from typing import List
from statistics import mean

def average(list: List[float]) -> float:
    if len(list) == 0:
        return 0.0
    return mean(list)

def weight_average(list: List[tuple]) -> float:
    if len(list) == 0:
        return 0.0

    top = 0.0
    bottom = 0.0
    for t in list:
        (v, w) = t
        if w <= 0:
            next
        top += (v*w)
        bottom += w

    if bottom <= 0:
        return 0.0

    return top/bottom

def float2digits(cls, v) -> float:
    if v is None:
        return 0
    return round(v, 2)

def float3digits(cls, v) -> float:
    if v is None:
        return 0
    return round(v, 3)

def ordinal(n: int):
    return str(n)+("th" if 4<=n%100<=20 else {1:"st",2:"nd",3:"rd"}.get(n%10, "th"))

def exists(list: List, t, e):
    for l in list:
        try:
            if l['t'] == e:
                return True
        except:
            pass
    return False

def init_cache(cache: dict):
    if cache is None:
        return cache

    if 'pilots' not in cache:
        cache['pilots'] = []

    if 'teams' not in cache:
        cache['teams'] = []

    if 'judges' not in cache:
        cache['judges'] = []

    if 'competitions' not in cache:
        cache['competitions'] = []

    if 'seasons' not in cache:
        cache['seasons'] = []

    if 'tricks' not in cache:
        cache['tricks'] = []

    return cache
