import os
import sys
import json

from datetime import date, timedelta


#
# utility methods 
#

def slugify(s):
    return s.replace(' ', '_').replace('/','_')

def get_value(obj, key):
    if key in obj:
        return obj[key]
    return None

def get_date_year(obj):
    if 'date' in obj:
        return obj['date'][0:4].strip()
    return ''

def get_eprint_id(obj):
    if 'eprint_id' in obj:
        return f"{obj['eprint_id']}"
    return ''

def get_object_type(obj):
    if 'type' in obj:
        return f'{obj["type"]}'
    return ''

def has_creator_ids(obj):
    for creator in obj['creators']:
        if 'id' in creator:
            return True
    return False

def make_label(s, sep = '_'):
    l = s.split(sep)
    for i, val in enumerate(l):
        l[i] = val.capitalize()
    return ' '.join(l)

def get_sort_name(o):
    if 'sort_name' in o:
        return o['sort_name']
    return ''

def get_sort_year(o):
    if 'year' in o:
        return o['year']
    return ''

def get_sort_subject(o):
    if 'subject_name' in o:
        return o['subject_name']
    return ''

def get_sort_publication(o):
    if ('publication' in o) and ('item' in publication['publication']):
        return o['publication']['item']
    return ''

def get_sort_collection(o):
    if ('collection' in o):
        return o['collection']
    return ''

def get_sort_event(o):
    if ('event_title' in o):
        return o['event_title'].strip()
    return ''


def get_lastmod_date(o):
    if ('lastmod' in o):
        return o['lastmod'][0:10]
    return ''

def get_sort_lastmod(o):
    if ('lastmod' in o):
        return o['lastmod']
    return ''

def get_sort_issn(o):
    if ('issn' in o):
        return o['issn']
    return ''

class Aggregator:
    """This class models the various Eprint aggregations used across Caltech Library repositories"""
    def __init__(self, c_name, objs):
        self.c_name = c_name
        self.objs = objs

    def aggregate_people(self):
        # now build our people list and create a people, eprint_id, title list
        people = {}
        for obj in self.objs:
            if has_creator_ids(obj):
                # For each author add a reference to object
                for creator in obj['creators']:
                    creator_id = creator['id']
                    creator_name = creator['display_name']
                    if not creator_id in people:
                        people[creator_id] = { 
                            'key': creator_id,
                            'label': creator_name,
                            'count' : 0,
                            'people_id': creator_id,
                            'sort_name': creator_name,
                            'objects' : []
                        }
                    people[creator_id]['count'] += 1
                    people[creator_id]['objects'].append(obj)
        # Now that we have a people list we need to sort it by name
        people_list = []
        for key in people:
            people_list.append(people[key])
        people_list.sort(key = get_sort_name)
        return people_list

    def aggregate_person_az(self):
        return self.aggregate_people()
    
    def aggregate_person(self):
        return self.aggregate_people()
    
    def aggregate_author(self):
        return self.aggregate_people()
    
    def aggregate_year(self):
        years = {}
        year = ''
        for obj in self.objs:
            if ('date' in obj):
                year = obj['date'][0:4].strip()
                if not year in years:
                    years[year] = { 
                        'key': str(year),
                        'label': str(year),
                        'count': 0,
                        'year': year, 
                        'objects': [] 
                    }
                years[year]['count'] += 1
                years[year]['objects'].append(obj)
        year_list = []
        for key in years:
            year_list.append(years[key])
        year_list.sort(key = get_sort_year, reverse = True)
        return year_list
    
    def aggregate_publication(self):
        publications = {}
        publication = ''
        for obj in self.objs:
            eprint_id = get_eprint_id(obj)
            year = get_date_year(obj)
            if ('publication' in obj):
                publication = obj['publication']
                key = slugify(publication)
                if not publication in publications:
                    publications[publication] = { 
                        'key': key,
                        'label': str(publication),
                        'count': 0,
                        'year': year, 
                        'objects': [] 
                    }
                publications[publication]['count'] += 1
                publications[publication]['objects'].append(obj)
        publication_list = []
        for key in publications:
            publication_list.append(publications[key])
        publication_list.sort(key = get_sort_publication)
        return publication_list

    def aggregate_issn(self):
        issns = {}
        issn = ''
        for obj in self.objs:
            eprint_id = get_eprint_id(obj)
            year = get_date_year(obj)
            if ('issn' in obj):
                issn = obj['issn']
                if not issn in issns:
                    issns[issn] = { 
                        'key': str(issn),
                        'label': str(issn),
                        'count': 0,
                        'year': year, 
                        'objects': [] 
                    }
                issns[issn]['count'] += 1
                issns[issn]['objects'].append(obj)
        issn_list = []
        for key in issns:
            issn_list.append(issns[key])
        issn_list.sort(key = get_sort_issn)
        return issn_list

    def aggregate_collection(self):
        collections = {}
        collection = ''
        for obj in self.objs:
            eprint_id = get_eprint_id(obj)
            year = get_date_year(obj)
            if ('collection' in obj):
                collection = obj['collection']
                key = slugify(collection)
                if not collection in collections:
                    collections[collection] = { 
                        'key': key,
                        'label': collection,
                        'count': 0,
                        'year': year, 
                        'objects': [] 
                    }
                collections[collection]['count'] += 1
                collections[collection]['objects'].append(obj)
        collection_list = []
        for key in collections:
            collection_list.append(collections[key])
        collection_list.sort(key = get_sort_collection)
        return collection_list

    def aggregate_event(self):
        events = {}
        event_title = ''
        for obj in self.objs:
            eprint_id = get_eprint_id(obj)
            year = get_date_year(obj)
            event_title = ''
            event_location = ''
            event_dates = ''
            if ('event_title' in obj):
                event_title = obj['event_title']
            if ('event_location' in obj):
                event_location = obj['event_location']
            if ('event_dates' in obj):
                event_dates = obj['event_dates']
            if not event_title in events:
                key = slugify(event_title)
                events[event_title] = { 
                    'key': key,
                    'label': event_title,
                    'count': 0,
                    'year': year, 
                    'objects': [] 
                }
            events[event_title]['count'] += 1
            events[event_title]['objects'].append(obj)
        event_list = []
        for key in events:
            event_list.append(events[key])
        event_list.sort(key = get_sort_event)
        return event_list

    def aggregate_subjects(self, subject_map):
        subjects = {}
        subject = ''
        for obj in self.objs:
            eprint_id = get_eprint_id(obj)
            year = get_date_year(obj)
    
            if ('subjects' in obj):
                for subj in obj['subjects']['items']:
                    subject_name = subject_map.get_subject(subj)
                    if subject_name != None:
                        if not subj in subjects:
                            subjects[subj] = { 
                                'key': subj,
                                'label': subject_name,
                                'count': 0,
                                'subject_id': subj, 
                                'subject_name': subject_name,
                                'objects': [] 
                            }
                        subjects[subj]['count'] += 1
                        subjects[subj]['objects'].append(obj)
        subject_list= []
        for key in subjects:
            subject_list.append(subjects[key])
        subject_list.sort(key = get_sort_subject)
        return subject_list

    def aggregate_ids(self):
        ids = {}
        for obj in self.objs:
            eprint_id = get_eprint_id(obj)
            if not eprint_id in ids:
                ids[eprint_id] = {
                    'key': eprint_id,
                    'label': eprint_id,
                    'eprint_id': eprint_id,
                    'count': 0,
                    'objects': []
                }
            ids[eprint_id]['count'] += 1
            ids[eprint_id]['objects'].append(obj)
        ids_list = []
        for key in ids:
            ids_list.append(ids[key])
        ids_list.sort(key = lambda x: int(x['key']))
        return ids_list
    
    def aggregate_types(self):
        types = {}
        for obj in self.objs:
            o_type = get_object_type(obj)
            label = make_label(o_type)
            if not o_type in types:
                types[o_type] = {
                    'key': o_type,
                    'label': label,
                    'type': o_type,
                    'count': 0,
                    'objects': []
                }
            types[o_type]['count'] += 1
            types[o_type]['objects'].append(obj)
        type_list = []
        for o_type in types:
            type_list.append(types[o_type])
        type_list.sort(key = lambda x: x['key'])
        return type_list

    def aggregate_latest(self):
        latest = {}
        today = date.today()
        td = timedelta(days = -7)
        seven_days_ago = (today - td).isoformat()
        for obj in self.objs:
            lastmod = get_lastmod_date(obj)
            if (lastmod != '') and (lastmod >= seven_days_ago):
                key = get_sort_lastmod(obj)
                year = get_date_year(obj)
                if not key in latest:
                    lastest[lastmod] = {
                        'key': key,
                        'label': lastmod,
                        'year': year,
                        'count': 0,
                        'objects': []
                    }
                latest[lastmod]['count'] += 1
                latest[lastmod]['objects'].append(obj)
        latest_list = []
        for key in latest:
            latest_list.append(latest[key])
        latest_list.sort(key = lambda x: x['key'], reverse = True)
        return latest_list
