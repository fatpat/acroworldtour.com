import classNames from "classnames";
import Link from "next/link";

import { components } from "@/types";

interface Props {
  season: components["schemas"]["SeasonExport"];
}

const SeasonCard = ({ season }: Props) => {
  const {
    code,
    name,
    image,
    number_of_pilots: numberOfPilots,
    number_of_teams: numberOfTeams,
    type,
    competitions,
  } = season;

  const competitionsCount = competitions.length;
  const compsPlural = competitionsCount > 1;
  const competitionsString = competitionsCount
    ? `${competitionsCount} competition${compsPlural ? "s" : ""}`
    : "No competitions data";

  const pilotsPlural = numberOfPilots > 1;
  const pilotsString = numberOfPilots
    ? `${numberOfPilots} pilot${pilotsPlural ? "s" : ""}`
    : "No pilot data";

  const teamsPlural = numberOfTeams > 1;
  const teamsString = numberOfTeams
    ? `${numberOfTeams} team${teamsPlural ? "s" : ""}`
    : "No team data";

  const seasonCover = image ?? "";

  return (
    <Link
      key={code}
      href={`/seasons/${code}`}
      className={classNames("max-w-lg flex-grow")}
    >
      <article
        style={{ backgroundImage: `url(${seasonCover})` }}
        className={classNames(
          "relative flex flex-col justify-between",
          "h-48 min-w-[240px] overflow-hidden rounded-xl p-4 text-white",
          "bg-black/60 bg-cover bg-center bg-no-repeat bg-blend-multiply",
          "shadow shadow-awt-dark-400",
          "hover:-translate-y-2 hover:bg-white/90 hover:text-current hover:bg-blend-screen",
          "hover:shadow-md",
          !seasonCover && "relative"
        )}
      >
        <h3 className="">{name}</h3>

        {!seasonCover && (
          <div
            className={classNames(
              "absolute inset-0 flex flex-grow overflow-hidden",
              "scale-105 rounded-xl mix-blend-overlay"
            )}
          >
            {competitions.map((competition) => {
              const { code, image } = competition;
              return (
                <div
                  key={code}
                  style={{ backgroundImage: `url(${image})` }}
                  className={classNames(
                    "h-full w-full scale-110",
                    "bg-cover bg-center bg-no-repeat"
                  )}
                />
              );
            })}
          </div>
        )}
        <footer>
          <p className="capitalize">{type}</p>
          <p className="">{competitionsString}</p>
          <p className="absolute bottom-4 right-4">
            {type === "solo" ? pilotsString : teamsString}
          </p>
        </footer>
      </article>
    </Link>
  );
};

export default SeasonCard;
