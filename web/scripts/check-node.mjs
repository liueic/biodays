if (process.versions.node.split(".")[0] !== "24") {
  console.error(
    `Biodays 需要 Node.js 24，当前为 ${process.version}。请先启用 .prototools 中的项目版本。`,
  );
  process.exit(1);
}
console.info(`Biodays Node.js runtime: ${process.version}`);
