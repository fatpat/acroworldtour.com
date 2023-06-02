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
    // number_of_teams: numberOfTeams,
    type,
    // competitions,
    country,
  } = season;

  return (
    <article className={className}>
      {image && (
        <Image
          src={image}
          alt="Competition Image"
          width={512}
          height={0}
          className="my-2 h-auto w-full rounded-xl"
        />
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
      )}
      <div>
        <h5 className="inline-block py-2 pl-4 text-left capitalize">Pilots:</h5>
        <p className="inline-block py-2 pl-4 text-left capitalize">
          {numberOfPilots}
        </p>
      </div>
      {/* {pilots.length > 0 && (
        <>
          <hr />
          {
            // <CompetitionJudges judges={competition.judges} className="pt-4 " />
          }
        </>
      )} */}
    </article>
  );
};

export default SeasonSummary;
