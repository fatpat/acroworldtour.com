import classNames from "classnames";
import Image from "next/image";

import { components } from "@/types";

interface Props {
  season: components["schemas"]["SeasonExport"];
  className?: string;
}

const SeasonSummary = ({ season, className }: Props) => {
  const {
    image,
    number_of_pilots: numberOfPilots,
    number_of_teams: numberOfTeams,
    type,
    competitions,
    country,
  } = season;

  return (
    <article className={className}>
      {image ? (
        <Image
          src={image}
          alt="Competition Image"
          width={512}
          height={0}
          className="my-2 h-auto w-full rounded-xl"
        />
      ) : (
        <div
          className={classNames(
            "flex aspect-video w-full flex-grow overflow-hidden",
            "rounded-xl"
          )}
        >
          {competitions.map((competition) => {
            const { code, image } = competition;
            return (
              <div
                key={code}
                style={{ backgroundImage: `url(${image})` }}
                className={classNames(
                  "h-full w-full",
                  "bg-cover bg-center bg-no-repeat"
                )}
              />
            );
          })}
        </div>
      )}
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">Type:</h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">{type}</p>
      </div>
      {country && (
        <div>
          <h5 className="inline-block py-2 pl-4 text-left capitalize">
            Country:
          </h5>
          <p className="inline-block py-2 pl-4 text-left capitalize">
            {country}
          </p>
        </div>
      )}{" "}
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">
          Competitions:
        </h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">
          {competitions.length}
        </p>
      </div>
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">
          {type === "solo" ? "Pilots:" : "Teams:"}
        </h5>
        <p className="inline-block py-2 pl-4 text-left">
          {type === "solo" ? numberOfPilots : numberOfTeams}
        </p>
      </div>
    </article>
  );
};

export default SeasonSummary;
