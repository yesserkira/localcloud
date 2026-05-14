interface Props {
  data: unknown;
}

export function JsonView({ data }: Props) {
  return (
    <pre className="json-view">
      {JSON.stringify(data, null, 2)}
    </pre>
  );
}
