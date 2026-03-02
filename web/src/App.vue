<template>
  <router-view />
</template>

<script setup lang="ts">
import { useAppStore } from '@/stores'
import { useI18n } from 'vue-i18n'
import { watch } from 'vue'

const appStore = useAppStore()
const { locale } = useI18n()

watch(() => appStore.locale, val => { locale.value = val }, { immediate: true })
</script>
