import classNames from "classnames";
import countries from "i18n-iso-countries";
import Link from "next/link";

import { components } from "@/types";

import SocialLink from "../ui/socialLink";

countries.registerLocale(require("i18n-iso-countries/langs/en.json"));

interface Props {
  pilot: components["schemas"]["PilotWithResults"];
}

const PilotDetails = ({ pilot }: Props) => {
  const {
    name,
    civl_link: civlLink,
    photo: photo,
    photo_highres: photoHighres,
    country,
    rank,
    about,
    social_links: socialLinks,
    sponsors,
    competitions_results: competitionsResults,
    seasons_results: seasonsResults,
  } = pilot;
  const alpha2country = countries
    .alpha3ToAlpha2(country?.toUpperCase())
    ?.toLowerCase();

  competitionsResults?.sort(
    (a, b) =>
      new Date(b.competition.start_date).getTime() -
        new Date(a.competition.start_date).getTime() || a.rank - b.rank
  );

  seasonsResults?.sort(
    (a, b) => b.season.year - a.season.year || a.rank - b.rank
  );

  return (
    <section
      className={classNames("flex w-full flex-col", "lg:w-1/2 lg:self-start")}
    >
      <Link
        href={civlLink}
        target="_blank"
        style={{ backgroundImage: `url('${photoHighres || photo}')` }}
        className={classNames(
          "aspect-square w-full bg-cover bg-center bg-no-repeat",
          "lg:fixed lg:right-0 lg:w-5/12"
        )}
      />
      <article className={classNames("flex w-full flex-col gap-4 px-4")}>
        <Link
          href={civlLink}
          target="_blank"
          className={classNames(
            "flex w-full max-w-sm flex-wrap items-baseline justify-between self-center px-4",
            "hover:bg-awt-accent-600 hover:text-white"
          )}
        >
          <h2 className="text-800">
            {name} {["🥇", "🥈", "🥉"][rank - 1]}
            <i
              className={classNames(
                country && alpha2country,
                "flag translate-x-2"
              )}
            />
          </h2>
          <h3 className="font-semibold">FAI Rank #{rank}</h3>
        </Link>
      </article>
      <article className="mt-8 p-4">
        <h4>About:</h4>
        <p className="font-medium">{about}</p>
      </article>
      {socialLinks.length > 0 && (
        <ul className="mx-auto flex w-full max-w-sm justify-evenly pb-6">
          {socialLinks.map((link) => {
            const { name: linkName, link: linkUrl } = link;
            return (
              <li key={linkName} className="hover:fill-awt-accent-500">
                <SocialLink link={linkUrl} media={linkName} />
              </li>
            );
          })}
        </ul>
      )}
      {sponsors.length > 0 && (
        <article className="p-4">
          <h4>Sponsors:</h4>
          <div className="flex flex-wrap justify-evenly">
            {sponsors.map((sponsor) => (
              <Link
                key={sponsor.name}
                href={sponsor.link}
                title={sponsor.name}
                style={{ backgroundImage: `url('${sponsor.img}')` }}
                target="_blank"
                className={classNames(
                  "m-4 aspect-video w-20 bg-contain bg-center bg-no-repeat",
                  "hover:fill-awt-accent-500"
                )}
              />
            ))}
          </div>
        </article>
      )}
      {((competitionsResults?.length ?? 0) > 0 ||
        (seasonsResults?.length ?? 0) > 0) && (
        <div className="mt-4 w-full bg-awt-dark-50 p-2">
          {(competitionsResults?.length ?? 0) > 0 && (
            <table className="w-full">
              <thead>
                <tr className="h-12">
                  <th className="pl-2 text-left">Competition</th>
                  <th className="pr-2 text-right">Rank</th>
                </tr>
              </thead>
              <tbody>
                {competitionsResults?.map(({ competition, rank }) => {
                  const { code, name } = competition;
                  return (
                    <tr
                      key={code}
                      className="h-8 cursor-pointer hover:bg-awt-dark-200"
                    >
                      <td className="pl-2">
                        {name} {["🥇", "🥈", "🥉"][rank - 1]}
                      </td>
                      <td className="pr-2 text-right">#{rank}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          )}
          {(seasonsResults?.length ?? 0) > 0 && (
            <table className="mt-4 w-full">
              <thead>
                <tr className="h-12">
                  <th className="pl-2 text-left">Season</th>
                </tr>
              </thead>
              <tbody>
                {seasonsResults &&
                  seasonsResults.map(({ season, rank }) => {
                    const { code, name } = season;
                    return (
                      <tr
                        key={code}
                        className="h-8 cursor-pointer hover:bg-awt-dark-200"
                      >
                        <td className="pl-2">
                          {name} {["🥇", "🥈", "🥉"][rank - 1]}
                        </td>
                        <td className="pr-2 text-right">#{rank}</td>
                      </tr>
                    );
                  })}
              </tbody>
            </table>
          )}
        </div>
      )}
    </section>
  );
};

export default PilotDetails;
