---
title: Upgrade your Tempo installation
menuTitle: Upgrade
description: Upgrade your Grafana Tempo installation to the latest version.
weight: 310
---

# Upgrade your Tempo installation

You can upgrade an existing Tempo installation to the next version. However, any new release has the potential to have breaking changes that should be tested in a non-production environment prior to rolling these changes to production.

The upgrade process changes for each version, depending upon the changes made for the subsequent release.

This upgrade guide applies to on-premise installations and not for Grafana Cloud.

>**TIP**: You can check your configuration options using the [`status` API endpoint]({{< relref "../api_docs#status" >}}) in your Tempo installation.

## Upgrade to Tempo 2.2

Tempo 2.2 has several considerations for any upgrade:

* vParquet2 is now the default block format
* Several configuration parameters have been renamed or removed.

For a complete list of changes, enhancements, and bug fixes, refer to the [Tempo 2.2 changelog](https://github.com/grafana/tempo/releases).

### Default block format changed to vParquet2

While not a breaking change, upgrading to Tempo 2.2 will by default change Tempo’s block format to vParquet2.

To stay on a previous block format, read the[Parquet configuration documentation]({{< relref "../configuration/parquet#choose-a-different-block-format" >}}).
We strongly encourage upgrading to vParquet2 as soon as possible as this is required for using structural operators in your TraceQL queries and provides query performance improvements, in particular on queries using the `duration` intrinsic.

### Updated JSonnet supports `statefulset` for the metrics-generator

Tempo 2.2 updates the `microservices` JSonnet to support a `statefulset` for the `metrics_generator` component.

{{% admonition type="note" %}}
This update is important if you use the experimental `local-blocks` processor.
{{% /admonition %}}

To support a new `processor`, the metrics-generator has been converted from a `deployment` into a `statefulset` with a PVC.
This requires manual intervention to migrate successfully and avoid downtime.
Note that currently both a `deployment` and a `statefulset` will be managed by the JSonnet for a period of time, after which we will delete the deployment from this repo and you will need to delete user-side references to the `tempo_metrics_generator_deployment`, as well as delete the deployment itself.

Refer to the PR for seamless migration instructions. [PRs [2533](https://github.com/grafana/tempo/pull/2533), [2467](https://github.com/grafana/tempo/pull/2467)]

### Removed or renamed configuration parameters

The following fields were removed or renamed.
| Parameter | Comments |
|---|---|
|<pre>query_frontend:<br>&nbsp;&nbsp;tolerate_failed_blocks: <int></pre> | Remove support for `tolerant_failed_blocks` [[PR 2416](https://github.com/grafana/tempo/pull/2416)] |
|<pre>storage:<br>&nbsp;&nbsp;trace:<br>&nbsp;&nbsp;s3:<br>&nbsp;&nbsp;insecure_skip_verify: true  // renamed to tls_insecure_skip_verify</pre> | Renamed `insecure_skip_verify` to `tls_insecure_skip_verify` [[PR 2407](https://github.com/grafana/tempo/pull/2407)] |


## Upgrade to Tempo 2.1

Tempo 2.1 has two major considerations for any upgrade:

* Support for search on v2 block is removed
* Breaking changes to metric names

For more information on other enhancements, read the [Tempo 2.1 release notes]({{< relref "../release-notes/v2-1" >}}).

### Remove support for Search on v2 blocks

Users can no longer search blocks in v2 format. Only vParquet and vParquet2 formats support search. The following search configuration options were removed from the overrides section:

```
overrides:
  max_search_bytes_per_trace:
  search_tags_allow_list:
  search_tags_deny_list:
```

The following metrics configuration was also removed:

```
tempo_ingester_trace_search_bytes_discarded_total
```

### Upgrade path to maintain search from Tempo 1.x to 2.1

Removing support for search on v2 blocks means that if you upgrade directly from 1.9 to 2.1, you will not be able to search your v2 blocks. To avoid this, upgrade to 2.0 first, since 2.0 supports searching both v2 and vParquet blocks. You can let your old v2 blocks gradually age out while Tempo creates new vParquet blocks from incoming traces. Once all of your v2 blocks have been deleted and you only have vParquet format-blocks, you can upgrade to Tempo 2.1. All of your blocks will be searchable.

Parquet files are no longer cached when carrying out searches.

### Breaking changes to metric names exposed by Tempo

All Prometheus metrics exposed by Tempo on its `/metrics` endpoint that were previously prefixed  with `cortex_` have now been renamed to be prefixed with `tempo_` instead. (PR [2204](https://github.com/grafana/tempo/pull/2204))

Tempo now includes SLO metrics to count where queries are returned within a configurable time range. (PR [2008](https://github.com/grafana/tempo/pull/2008))

The ``query_frontend_result_metrics_inspected_bytes`` metric was removed in favor of ``query_frontend_bytes_processed_per_second`.`


## Upgrade from Tempo 1.5 to 2.0

Tempo 2.0 marks a major milestone in Tempo’s development. When planning your upgrade, consider these factors:

- Breaking changes:
  - Renamed, removed, and moved configurations are described in section below.
  - The `TempoRequestErrors` alert was removed from mixin. Any Jsonnet users relying on this alert should copy this into their own environment.
- Advisory:
  - Changed defaults – Are these updates relevant for your installation?
  - TraceQL editor needs to be enabled in Grafana to use the query editor.
  - Resource requirements have changed for Tempo 2.0 with the default configuration.

Once you upgrade to Tempo 2.0, there is no path to downgrade.

>**Note**: There is a potential issue loading Tempo 1.5's experimental Parquet storage blocks. You may see errors or even panics in the compactors. We have only been able to reproduce this with interim commits between 1.5 and 2.0, but if you experience any issues please [report them](https://github.com/grafana/tempo/issues/new?assignees=&labels=&template=bug_report.md&title=) so we can isolate and fix this issue.

### Check Tempo installation resource allocation

Parquet provides faster search and is required to enable TraceQL. However, the Tempo installation will require additional CPU and memory resources to use Parquet efficiently. Parquet is more costly due to the extra work of building the columnar blocks, and operators should expect at least 1.5x increase in required resources to run a Tempo 2.0 cluster. Most users will find these extra resources are negligible compared to the benefits that come from the additional features of TraceQL and from storing traces in an open format.

You can can continue using the previous `v2` block format using the instructions provided in the [Parquet configuration documentation]({{< relref "../configuration/parquet" >}}). Tempo will continue to support trace by id lookup on the `v2` format for the foreseeable future.

### Enable TraceQL in Grafana

TraceQL is enabled by default in Tempo 2.0. The TraceQL query editor requires Grafana 9.3.2 and later.

The TraceQL query editor is in beta in Grafana 9.3.2 and needs to be enabled with the `traceqlEditor` feature flag.

### Check configuration options for removed and renamed options

The following tables describe the parameters that have been removed or renamed.

#### Removed and replaced

| Parameter | Comments |
| --- | --- |
| <pre>query_frontend:<br>&nbsp;&nbsp;query_shards:</pre> | Replaced by `trace_by_id.query_shards`. |
| <pre>querier:<br>&nbsp;&nbsp;query_timeout:</pre> | Replaced by two different settings: `search.query_timeout` and `trace_by_id.query_timeout`. |
| <pre>ingester:<br>&nbsp;&nbsp;use_flatbuffer_search:</pre> | Removed and automatically determined based on block format. |
| `search_enabled` | Removed. Now defaults to true. |
| `metrics_generator_enabled` | Removed. Now defaults to true. |

#### Renamed

The following `compactor` configuration parameters were renamed.

| Parameter | Comments |
| --- | --- |
| <pre>compaction:<br>&nbsp;&nbsp;chunk_size_bytes:</pre> | Renamed to `v2_in_buffer_bytes` |
| <pre>compaction:<br>&nbsp;&nbsp;flush_size_bytes:</pre> | Renamed to `v2_out_buffer_bytes` |
| <pre>compaction:<br>&nbsp;&nbsp;iterator_buffer_size:</pre> | Renamed to `v2_prefetch_traces_count` |

The following `storage` configuration parameters were renamed.

| Parameter | Comments |
| --- | --- |
| <pre>wal:<br>&nbsp;&nbsp;encoding:</pre> | Renamed to `v2_encoding` |
| <pre>block:<br>&nbsp;&nbsp;index_downsample_bytes:</pre> | Renamed to `v2_index_downsample_bytes` |
| <pre>block:<br>&nbsp;&nbsp;index_page_size_bytes:</pre> | Renamed to `v2_index_page_size_bytes` |
| <pre>block:<br>&nbsp;&nbsp;encoding:</pre> | Renamed to `v2_encoding` |
| <pre>block:<br>&nbsp;&nbsp;row_group_size_bytes:</pre> | Renamed to `parquet_row_group_size_bytes` |

The Azure Storage configuration section now uses snake case with underscores (`_`) instead of dashes (`-`). Example of using snake case on Azure Storage config:

```yaml
# config.yaml
storage:
  trace:
    azure:
      storage_account_name:
      storage_account_key:
      container_name:
```
