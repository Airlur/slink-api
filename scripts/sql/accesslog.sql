-- 访问日志模板表（access_logs_template）
CREATE TABLE IF NOT EXISTS `access_logs_template` (
  `id` BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '日志唯一标识',
  `short_code` VARCHAR(20) NOT NULL COMMENT '关联短码',
  `user_id` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '访问者用户 ID，匿名访问为 0',
  `ip` VARCHAR(45) NOT NULL COMMENT '访问 IP',
  `user_agent` VARCHAR(512) NULL DEFAULT NULL COMMENT '访问 User-Agent',
  `device_type` VARCHAR(20) NULL DEFAULT NULL COMMENT '设备类型',
  `os_version` VARCHAR(50) NULL DEFAULT NULL COMMENT '操作系统版本',
  `browser` VARCHAR(50) NULL DEFAULT NULL COMMENT '浏览器',
  `country` VARCHAR(80) NULL DEFAULT NULL COMMENT '国家',
  `province` VARCHAR(50) NULL DEFAULT NULL COMMENT '省份/州',
  `city` VARCHAR(50) NULL DEFAULT NULL COMMENT '城市',
  `channel` VARCHAR(100) NULL DEFAULT NULL COMMENT '访问来源渠道',
  `accessed_at` DATETIME NOT NULL COMMENT '访问时间',
  PRIMARY KEY (`id`),
  INDEX `idx_short_code_accessed_at` (`short_code`, `accessed_at`),
  INDEX `idx_accessed_at` (`accessed_at`),
  INDEX `idx_province_city` (`province`, `city`),
  INDEX `idx_country_province_city` (`country`, `province`, `city`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='访问日志模板表';
