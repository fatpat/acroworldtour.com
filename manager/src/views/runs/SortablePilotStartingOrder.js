import React from 'react';
import {useSortable} from '@dnd-kit/sortable';
import {CSS} from '@dnd-kit/utilities';
import ListItem from "@mui/material/ListItem"
import ListItemIcon from "@mui/material/ListItemIcon"
import ListItemText from "@mui/material/ListItemText"
import ListItemSecondaryAction from "@mui/material/ListItemSecondaryAction"
import DragHandleIcon from "@mui/icons-material/DragHandle"

const SortablePilotStartingOrder = (props) => {
  const {
    attributes,
    listeners,
    setNodeRef,
    transform,
    transition,
  } = useSortable({id: props.id});
  
  const style = {
    transform: CSS.Transform.toString(transform),
    transition,
  };
  
  return (
    <ListItem ref={setNodeRef} style={style} {...attributes} {...listeners}>
      <ListItemIcon className="hideToPrint"><DragHandleIcon /></ListItemIcon>
      <ListItemText primary={props.text} />
    </ListItem>
  );
}

export default SortablePilotStartingOrder
