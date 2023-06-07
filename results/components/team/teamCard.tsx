import classNames from "classnames";
import { alpha3ToAlpha2 } from "i18n-iso-countries";
import Link from "next/link";
import { preload } from "swr";

import { API_URL } from "@/constants";
import { components } from "@/types";
import { fetcher } from "@/utils/fetcher";

interface Props {
  team: components["schemas"]["TeamExport"];
}

const TeamCard = ({ team }: Props) => {
  const { _id: teamId, name, pilots } = team;

  return (
    <Link
      key={teamId}
      href={`/teams/${teamId}/${name}`}
      className={classNames("max-w-lg flex-1", "min-w-[300px]")}
      onMouseEnter={() => preload(`${API_URL}/teams/${teamId}`, fetcher)}
    >
      <article
        className={classNames(
          "relative flex flex-col justify-between",
          "h-48 overflow-hidden rounded-xl py-2 text-white",
          "bg-awt-dark-950/60 bg-cover bg-center bg-no-repeat bg-blend-overlay",
          "shadow shadow-awt-dark-600",
          "hover:-translate-y-1 hover:bg-gradient-radial hover:from-awt-accent-400/50 hover:to-awt-accent-950/50 hover:bg-blend-screen",
          "hover:shadow-md",
        )}
      >
        <h3 className="capitalize">{name}</h3>

        <div
          className={classNames(
            "absolute inset-0 flex flex-grow overflow-hidden",
            "rounded-xl mix-blend-overlay",
            "hover:blur-sm",
          )}
        >
          {pilots.map((pilot) => {
            const { civlid, photo } = pilot;
            return (
              <div
                key={civlid}
                style={{ backgroundImage: `url(${photo})` }}
                className={classNames(
                  "h-full w-full",
                  "bg-cover bg-center bg-no-repeat",
                )}
              />
            );
          })}
        </div>
        <footer>
          <div className="flex">
            {pilots.map((pilot) => {
              const alpha2country = alpha3ToAlpha2(
                pilot.country?.toUpperCase(),
              )?.toLowerCase();
              return (
                <p
                  key={pilot.civlid}
                  className="flex flex-1 items-center justify-between px-4"
                >
                  {pilot.name}
                  {["🥇", "🥈", "🥉"][pilot.rank - 1]}
                  <i
                    className={classNames(
                      pilot.country && alpha2country,
                      "flag pl-2",
                    )}
                  />
                </p>
              );
            })}
          </div>
          <div className="flex">
            {pilots.map((pilot) => (
              <div
                key={pilot.civlid}
                className="flex w-full items-center justify-between px-4"
              >
                <p>FAI Rank:</p>
                <p className="">#{pilot.rank}</p>
              </div>
            ))}
          </div>
        </footer>
      </article>
    </Link>
  );
};

export default TeamCard;
