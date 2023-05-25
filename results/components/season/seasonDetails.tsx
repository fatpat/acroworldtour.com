import classNames from "classnames";

import { components } from "@/types";

interface Props {
  season: components["schemas"]["SeasonExport"];
}

const CompetitionDetails = ({ season }: Props) => {
  const { code, name, competitions, image, results, number_of_pilots, year } =
    season;

  return (
    <div
      style={{ backgroundImage: `url('${image}')` }}
      className={classNames(
        "bg-white/80 bg-cover bg-center bg-no-repeat bg-blend-overlay",
        "mt-4 flex w-full flex-wrap justify-evenly gap-6 px-4"
      )}
    >
      <h2>{name}</h2>
      <div
        className={classNames(
          "mt-4 flex w-full flex-wrap justify-evenly gap-6 px-4"
        )}
      >
        <section>
          <h3>Overall Results</h3>
          <table>
            <thead>
              <tr>
                <th>Pilot</th>
                <th>Score</th>
              </tr>
            </thead>
            <tbody>
              {/* {results.overall_results.map((result) => {
                const { pilot, score, result_per_run } = result;
                const roundedScore = Math.round(score * 100) / 100;
                return (
                  <tr key={pilot?.name}>
                    <td>{pilot?.name}</td>
                    <td className="text-right">{roundedScore}</td>
                  </tr>
                );
              })} */}
            </tbody>
          </table>
        </section>
        <section>
          <article className={classNames()}>
            <h3>Article</h3>
          </article>
          <article className={classNames()}>
            <h3>Article</h3>
          </article>
        </section>
        <section>
          <article className={classNames()}>
            <h3>Article</h3>
          </article>
          <article className={classNames()}>
            <h3>Article</h3>
          </article>
        </section>
      </div>
    </div>
  );
};

export default CompetitionDetails;
