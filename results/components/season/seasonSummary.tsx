import classNames from "classnames";
import Image from "next/image";

import { components } from "@/types";

interface Props {
  season: components["schemas"]["SeasonExport"];
  className?: string;
}

const SeasonSummary = ({ season, className }: Props) => {
  const {
    image: seasonImage,
    number_of_pilots: numberOfPilots,
    number_of_teams: numberOfTeams,
    type,
    competitions,
    country,
  } = season;

  const numberOfCompetitions = competitions.length;

  return (
    <article className={className}>
      <header
        className={classNames(
          "relative flex aspect-video w-full flex-grow overflow-hidden",
          "rounded-xl"
        )}
      >
        {seasonImage ? (
          <Image
            src={seasonImage}
            alt="Competition Image"
            fill
            className="h-auto w-full rounded-xl"
          />
        ) : (
          competitions.map((competition) => {
            const { code, image: competitionImage } = competition;
            return (
              <div
                key={code}
                style={{ backgroundImage: `url(${competitionImage})` }}
                className={classNames(
                  "h-full w-full",
                  "bg-cover bg-center bg-no-repeat"
                )}
              />
            );
          })
        )}
      </header>
      <section className="grid gap-2 pb-2 pl-5 pt-4">
        <div className="flex items-baseline gap-1">
          <h5>Type:</h5>
          <p>{type}</p>
        </div>
        {country && (
          <div className="flex items-baseline gap-1">
            <h5>Country:</h5>
            <p>{country}</p>
          </div>
        )}
        <div className="flex items-baseline gap-1">
          <h5>{type === "solo" ? "Pilots:" : "Teams:"}</h5>
          <p>{type === "solo" ? numberOfPilots : numberOfTeams}</p>
        </div>
        <h5 className="text-left">
          {numberOfCompetitions
            ? `${numberOfCompetitions} Competition${
                numberOfCompetitions > 1 ? "s" : ""
              }:`
            : "No competitions yet"}
        </h5>
        <ul className="-mt-1">
          {competitions.map((competition) => {
            const { name, code } = competition;
            return (
              <li key={code}>
                <a href={`/competitions/${code}`}>
                  <p
                    title={`Navigate to the main page of ${name}`}
                    className="hover:font-bold"
                  >
                    🔹{name}
                  </p>
                </a>
              </li>
            );
          })}
        </ul>
      </section>
    </article>
  );
};

export default SeasonSummary;
