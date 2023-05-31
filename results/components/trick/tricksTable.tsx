import { components } from "@/types";

interface Props {
  tricks: components["schemas"]["Trick"][];
  className?: string;
}

const TricksTable = ({ tricks, className }: Props) => {
  return (
    <article className={className}>
      <h4 className="col-span-6 col-start-1 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
        Trick
      </h4>
      <h4 className="col-span-1 col-start-7 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
        Acronym
      </h4>
      <h4 className="col-span-1 col-start-8 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
        Properties
      </h4>
      <h4 className="col-span-2 col-start-9 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
        Bonuses
      </h4>
      <h4 className="col-span-1 col-start-11 border-[1px] border-awt-dark-500 bg-awt-dark-900 py-1 text-white">
        Technical Coefficient
      </h4>
      {tricks
        .sort((a, b) => a.technical_coefficient - b.technical_coefficient)
        .map((trick) => (
          <>
            <p className="col-span-6 col-start-1 border-[1px] py-2 pl-2 text-left">
              {trick.sample_video ? (
                <a href={trick.sample_video} target="_blank">
                  {trick.name} 📺
                </a>
              ) : (
                trick.name
              )}
            </p>
            <p className="col-span-1 col-start-7 border-[1px] py-2 pl-2 text-center">
              {trick.acronym}
            </p>
            <p className="col-span-1 col-start-8 border-[1px] py-2 pl-2 text-center">
              {trick.directions.includes("left") && (
                <label htmlFor={`directions-${trick._id}`} title="Right">
                  ⬅️
                </label>
              )}
              {trick.directions.includes("right") && (
                <label htmlFor={`directions-${trick._id}`} title="Left">
                  ➡️
                </label>
              )}
              {(trick.directions.includes("opposite") ||
                trick.directions.includes("mirror")) && (
                <label
                  htmlFor={`directions-${trick._id}`}
                  title="Opposite/Mirror"
                >
                  ↔️
                </label>
              )}
              {trick.repeatable && (
                <label
                  htmlFor={`repeatable-${trick._id}`}
                  title="this trick can be repeated"
                >
                  🔁
                </label>
              )}
              {(trick.first_maneuver || 0) > 0 && (
                <label
                  htmlFor={`first-maneuver-${trick._id}`}
                  title="this trick must be performed as a first trick only"
                >
                  ①
                </label>
              )}
              {(trick.no_last_maneuver || 0) > 0 && (
                <label
                  htmlFor={`no-last-maneuver-${trick._id}`}
                  title={`this trick cannot be performed as a last ${trick.no_last_maneuver} maneuver(s)`}
                >
                  ⨂
                </label>
              )}
            </p>
            <p className="col-span-2 col-start-9 border-[1px] py-2 pl-2 text-center">
              {trick.bonuses
                .sort((a, b) => a.bonus - b.bonus)
                .map((bonus) => (
                  <p key={`${trick.name}-${bonus.name}`}>
                    {bonus.name} ({bonus.bonus}%)
                    {bonus.sample_video && (
                      <a href={bonus.sample_video} target="_blank">
                        {" "}
                        📺
                      </a>
                    )}
                  </p>
                ))}
            </p>
            <p className="py-2u col-span-1 col-start-11 border-[1px] pl-2 text-right">
              {trick.technical_coefficient.toFixed(2)}
            </p>
          </>
        ))}
    </article>
  );
};

export default TricksTable;
