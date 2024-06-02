import classNames from "classnames";
import Link from "next/link";

import { components } from "@/types";

interface Props {
  season: components["schemas"]["SeasonExportLight"];
}

const SeasonCard = ({ season }: Props) => {
  const {
    code,
    name,
    image,
    number_of_pilots: numberOfPilots,
    number_of_teams: numberOfTeams,
    number_of_competitions: numberOfCompetitions,
    type,
  } = season;

  const compsPlural = numberOfCompetitions > 1;
  const competitionsString = numberOfCompetitions
    ? `${numberOfCompetitions} competition${compsPlural ? "s" : ""}`
    : "";

  const pilotsPlural = numberOfPilots > 1;
  const pilotsString = numberOfPilots
    ? `${numberOfPilots} pilot${pilotsPlural ? "s" : ""}`
    : "";

  const teamsPlural = numberOfTeams > 1;
  const teamsString = numberOfTeams
    ? `${numberOfTeams} team${teamsPlural ? "s" : ""}`
    : "";

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
          "bg-awt-dark-900/60 bg-cover bg-center bg-no-repeat bg-blend-multiply",
          "shadow shadow-awt-dark-400",
          "hover:-translate-y-2 hover:bg-white/90 hover:text-current hover:bg-blend-screen",
          "hover:shadow-md",
          !seasonCover && "relative",
        )}
      >
        <h3 className="">{name}</h3>
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
