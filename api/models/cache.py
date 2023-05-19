import logging
log = logging.getLogger(__name__)

class Cache:

    def __init__(self):
        self.cache = {}
        self.cache['pilots'] = []
        self.cache['teams'] = []
        self.cache['judges'] = []
        self.cache['tricks'] = []
        self.cache['competitions'] = []
        self.cache['seasons'] = []
        self.cache['files'] = []

        self.all = {}
        self.all['pilots'] = []
        self.all['teams'] = []
        self.all['judges'] = []
        self.all['tricks'] = []
        self.all['competitions'] = []
        self.all['seasons'] = []
        self.all['files'] = []

    def get(self, type, key):
        for e in self.cache[type]:
            if type == 'pilots':
                if e.id == key or e.name == str(key):
                    return e
            elif type == 'teams':
                if str(e.id) == str(key) or e.name == str(key):
                    return e
            elif type == 'judges':
                if str(e.id) == str(key) or e.name == str(key):
                    return e
            elif type == 'tricks':
                if str(e.id) == str(key) or e.name == str(key) or e.acronym == str(key):
                    return e
            elif type == 'competitions':
                if str(e.id) == str(key) or e.name == str(key) or e.code == str(key):
                    return e
            elif type == 'seasons':
                if str(e.id) == str(key) or e.name == str(key) or e.code == str(key):
                    return e
            elif type == 'files':
                if str(e.id) == str(key):
                    return e
        return None

    def exists(self, type, key):
        return (self.get(type, key) is not None)

    def add(self, type, element):
        if self.exists(type, element.id):
            return
        self.cache[type].append(element)

    def set_all(self, type, elements):
        self.all[type] = elements
        for element in elements:
            self.add(type, element)

    def get_all(self, type):
        if len(self.all[type]) == 0:
            return None
        return self.all[type]

    def clean(self, type):
        self.cache[type] = []
        self.all[type] = []
