import { API_URL } from '@/constants';

export const ordinalSuffixOf = (n: number) => {
  if (n % 10 == 1 && n % 100 != 11) return n + 'st';
  if (n % 10 == 2 && n % 100 != 12) return n + 'nd';
  if (n % 10 == 3 && n % 100 != 13) return n + 'rd';
  return n + 'th';
};

export const relativeToUrl = (u: string | undefined) => {
  if (typeof u !== 'string') return '';
  if (u.startsWith('/')) return API_URL + u;
  return u;
};
