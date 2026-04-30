interface Props {
  title: string;
  children?: React.ReactNode;
  button?: React.ReactNode;
}

function ServiceFormCell({ title, children, button }: Props) {
  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        flexWrap: "wrap",
        width: "100%",
        padding: 18,
        margin: 0,
      }}
    >
      <div
        style={{
          display: "flex",
          flexDirection: "row",
          justifyContent: "space-between",
          alignItems: "start",
          width: "100%",
          marginBottom: 18,
        }}
      >
        <h1
          style={{
            fontSize: 16,
            fontWeight: "bold",
          }}
        >
          {title}
        </h1>

        {button}
      </div>
      <div
        style={{
          display: "flex",
          width: "100%",
          gap: 18,
          alignItems: "end",
        }}
      >
        {children}
      </div>
    </div>
  );
}

export default ServiceFormCell;
