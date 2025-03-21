---
title: Apache Parquet block format
menuTitle: Apache Parquet
description: Learn about Tempo's Parquet block format.
weight: 75
---

# Apache Parquet block format


Tempo has a default columnar block format based on Apache Parquet. This format is required for tags-based search as well as [TraceQL]({{< relref "../traceql" >}}), the query language for traces. The columnar block format improves search performance and enables a large ecosystem of tools to access the underlying trace data.

For more information, refer to the [Parquet schema]({{< relref "../operations/schema" >}}) and the [Parquet design document](https://github.com/mdisibio/tempo/blob/design-proposal-parquet/docs/design-proposals/2022-04%20Parquet.md).

If you install using the new Helm charts, then Parquet is enabled by default.

## Considerations

The Parquet block format is enabled by default in Tempo 2.0. No data conversion or upgrade process is necessary. As soon as the format is enabled, Tempo starts writing data in that format, leaving existing data as-is.

Block formats based on Parquet require more CPU and memory resources than the previous `v2` format but provide search and TraceQL functionality.

## Choose a different block format

The default block format is `vParquet2`, which is the latest iteration of Tempo's Parquet based columnar block format.
It is still possible to use the previous format `vParquet`.
To enable it, set the block version option to `vParquet` in the Storage section of the configuration file.

Since v2.2, `vParquet3` is also available.
It introduces dedicated columns, which improve query performance by storing attributes in dedicated columns,
rather than in the generic attribute key-value list.
For more information, see [Dedicated columns]({{< relref "../operations/tempo_cli" >}}).

```yaml
# block format version. options: v2, vParquet, vParquet2, vParquet3
[version: vParquet]
```

It is possible to disable Parquet and use the previous `v2` block format. This disables all forms of search, but also reduces resource consumption, and may be desired for a high-throughput cluster that does not need these capabilities. Set the block version option to `v2` in the Storage section of the configuration file.

```yaml
# block format version. options: v2, vParquet, vParquet2, vParquet3
[version: v2]
```

To re-enable the default `vParquet2` format, remove the block version option from the Storage section of the configuration file or set the option to `vParquet2`.

## Parquet configuration parameters

Some parameters in the Tempo configuration are specific to Parquet.
For more information, refer to the [storage configuration documentation]({{< relref "../configuration#storage" >}}).

### Trace search parameters

These configuration options impact trace search.

| Parameter | Default value | Description |
| --- | --- | --- |
| `[read_buffer_size_bytes: <int>]` | `10485676` | Size of read buffers used when performing search on a vParquet block. This value times the `read_buffer_count`  is the total amount of bytes used for buffering when performing search on a Parquet block.
 |
| `[read_buffer_count: <int>]` | 32 | Number of read buffers used when performing search on a vParquet block. This value times the `read_buffer_size_bytes` is the total amount of bytes used for buffering when performing search on a Parquet block.
 |

The `cache_control` section contains the follow parameters for Parquet metadata objects:

| Parameter | Default value | Description |
| --- | --- | --- |
| <code>[footer: <bool> \| default = false]</code> | `false` | Specifies if the footer should be cached |
| `[column_index: <bool> \| default = false]` | `false` | Specifies if the column index should be cached |
| `[offset_index: <bool> \| default = false]` | `false` | Specifies if the offset index should be cached |

## Convert to Parquet

If you have used an earlier version of the Parquet format, you can use `tempo-cli` to convert a Parquet file from its existing schema to the one used in Tempo 2.0.

For instructions, refer to the [Parquet convert command documentation]({{< relref "../operations/tempo_cli#parquet-convert-command" >}}).
