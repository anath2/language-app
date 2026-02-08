<script lang="ts">
interface Props {
  message: string;
}

const { message }: Props = $props();
</script>

<div class="error-message">
  {message}
</div>
