interface Props {
  message: string;
  action?: React.ReactNode;
}

export function EmptyState({ message, action }: Props) {
  return (
    <div className="empty-state">
      <p>{message}</p>
      {action}
    </div>
  );
}
