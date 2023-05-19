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
    civlid,
    name,
    civl_link: civlLink,
    photo,
    country,
    rank,
    about,
    background_picture: coverPicture,
    social_links: socialLinks,
    sponsors,
    competitions_results: competitionsResults,
    seasons_results: seasonsResults,
  } = pilot;
  const alpha2country = countries
    .alpha3ToAlpha2(country.toUpperCase())
    .toLowerCase();
  const countryName = countries.getName(country, "en");

  return (
    <>
      <Link
        href={photo}
        target="_blank"
        style={{ backgroundImage: `url('${photo}')` }}
        className={classNames(
          "aspect-square w-full bg-cover bg-center bg-no-repeat",
          "lg:fixed lg:right-0 lg:w-5/12"
        )}
      ></Link>
      <article
        className={classNames(
          "flex w-full flex-col items-center gap-6",
          "sm:px-8",
          "lg:w-6/12"
        )}
      >
        <div
          className={classNames(
            "flex w-full flex-col items-baseline justify-between",
            "sm:flex-row"
          )}
        >
          <h3>{name}</h3>
          <h4>
            <Link
              href={civlLink}
              target="_blank"
              className="text-sm text-cyan-600 hover:underline"
            >
              CIVL ID:{" "}
              <small className="ml-2 text-sm font-medium">{civlid}</small>
            </Link>
          </h4>
        </div>
        <div
          className={classNames(
            "-mt-6 flex w-full flex-col items-baseline justify-between",
            "sm:flex-row"
          )}
        >
          <h4>
            {rank === 9999 ? "Unranked" : `Overall Rank: `}
            <span className="ml-2 font-medium">{`#${rank}`}</span>
          </h4>{" "}
          <h4>
            Country: <span className="ml-4 font-medium">{countryName}</span>
            <span className="ml-2">
              <i className={classNames(alpha2country, "flag translate-x-1")} />
            </span>
          </h4>
        </div>
        <div className="rounded-xl bg-awtgrey-100 p-4">
          <h4>About:</h4>
          <p className="font-medium">{about}</p>
        </div>
        {socialLinks.length > 0 && (
          <div className="flex w-full justify-center gap-4">
            {socialLinks.map((link) => {
              const { name: linkName, link: linkUrl } = link;
              return (
                <SocialLink key={linkName} link={linkUrl} media={linkName} />
              );
            })}
          </div>
        )}
        {sponsors.length > 0 && (
          <div className="mb-4 w-full rounded-xl bg-awtgrey-100 p-4">
            <h4>Sponsors:</h4>
            {sponsors.map((sponsor) => (
              <Link
                key={sponsor.name}
                href={sponsor.link}
                target="_blank"
                className="text-cyan-600 hover:underline"
              >
                <em>{sponsor.name}</em>
                <p className="font-medium"></p>
              </Link>
            ))}
          </div>
        )}
        <div className="w-full rounded-xl bg-awtgrey-100 p-2">
          <table className="w-full">
            <thead>
              <tr>
                <th className="pl-2 text-left">Competition</th>
                <th className="pr-2 text-right">Rank</th>
              </tr>
            </thead>
            <tbody>
              {competitionsResults?.map(({ competition, rank }) => {
                const { code, name, image } = competition;
                return (
                  <tr
                    key={code}
                    // style={{ backgroundImage: `url('${image}')` }}
                    className="cursor-pointer hover:bg-awtgrey-200"
                  >
                    <td className="pl-2">{name}</td>
                    <td className="pr-2 text-right">#{rank}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
          <table className="mt-4 w-full">
            <thead>
              <tr>
                <th className="pl-2 text-left">Season</th>
              </tr>
            </thead>
            <tbody>
              {seasonsResults?.map(({ season, rank }) => {
                const { code, name, image } = season;
                return (
                  <tr
                    key={code}
                    // style={{ backgroundImage: `url('${image}')` }}
                    className="cursor-pointer hover:bg-awtgrey-200"
                  >
                    <td className="pl-2">{name}</td>
                    <td className="pr-2 text-right">#{rank}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      </article>
    </>
  );
};

export default PilotDetails;
