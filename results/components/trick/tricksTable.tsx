import { Checkbox,Col,Row, Table } from "@nextui-org/react";
import classNames from "classnames";
import Link from "next/link";

import PilotCard from "@/components/pilot/pilotCard";
import { components } from "@/types";

interface Props {
  tricks: components["schemas"]["Trick"][];
}

const TricksTable = ({ tricks }: Props) => {
/*
 */
  return (
      <Table>
        <Table.Header>
          <Table.Column align="center">Trick</Table.Column>
          <Table.Column align="center">Acronym</Table.Column>
          <Table.Column align="center">Properties</Table.Column>
          <Table.Column align="center">Bonuses</Table.Column>
          <Table.Column align="center">Technical Coefficient</Table.Column>
        </Table.Header>
        <Table.Body>
          {tricks.sort((a,b) => a.technical_coefficient - b.technical_coefficient).map((trick) => (
            <Table.Row key="{trick._id}">
              <Table.Cell>{trick.name}</Table.Cell>
              <Table.Cell>{trick.acronym}</Table.Cell>
              <Table.Cell>
                {trick.directions.includes("left") && <label title="Right">⬅️</label>}
                {trick.directions.includes("right") && <label title="Left">➡️</label>}
                {(trick.directions.includes("opposite") || trick.directions.includes("mirror")) && <label title="Opposite/Mirror">↔️</label>}
                {trick.repeatable && <label title="this trick can be repeated">🔁</label>}
                {(trick.first_maneuver || 0) > 0 && <label title="this trick must be performed as a first trick only">①</label>}
                {(trick.no_last_maneuver || 0) > 0 && <label title={`this trick cannot be performed as a last ${trick.no_last_maneuver} trick(s) of a run`}>⨂</label>}
              </Table.Cell>
              <Table.Cell>
                {trick.bonuses.sort((a,b) => a.bonus - b.bonus).map(bonus => (
                  <Row key="{trick._id}-{bonus.name}">
                    <Col>{bonus.name}</Col>
                    <Col>{bonus.bonus}%</Col>
                  </Row>
                ))}
              </Table.Cell>
              <Table.Cell>{trick.technical_coefficient.toFixed(2)}</Table.Cell>
            </Table.Row>
          ))}
        </Table.Body>
      </Table>
  );
};

export default TricksTable;
