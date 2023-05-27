import classNames from "classnames";
import Image from "next/image";

import { components } from "@/types";
import { capitalise } from "@/utils/capitalise";

interface Props {
  competition: components["schemas"]["CompetitionPublicExport"];
}

const CompetitionSummary = ({ competition }: Props) => {
  const {
    end_date: endDate,
    image,
    location,
    number_of_pilots: numberOfPilots,
    start_date: startDate,
    type,
  } = competition;

  return (
    <section
      className={classNames(
        "flex min-w-max flex-col gap-2 rounded-xl bg-awt-dark-50",
        "lg:p-4"
      )}
    >
      {image && (
        <Image
          src={image}
          alt="Competition Image"
          width={512}
          height={0}
          className="my-2 h-auto w-full rounded-xl"
        />
      )}
      <p>{`Type: ${capitalise(type)}`}</p>
      <p>{`Location: ${location}`}</p>
      <p>{`Pilots: ${numberOfPilots}`}</p>
      <p>{`Start Date: ${startDate}`}</p>
      <p>{`End Date: ${endDate}`}</p>
    </section>
  );
};

export default CompetitionSummary;
