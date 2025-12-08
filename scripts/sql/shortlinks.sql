-- 创建短链接表
CREATE TABLE IF NOT EXISTS `shortlinkss` (
  `id` BIGINT NOT NULL AUTO_INCREMENT COMMENT '短链接唯一标识',
  `short_code` VARCHAR(20) NOT NULL COMMENT '短码（Base62，区分大小写）',
  `original_url` TEXT NOT NULL COMMENT '原始长链接（URLEncode编码）',
  `original_url_md5` VARCHAR(32) NOT NULL COMMENT '原始长链接的MD5摘要',
  `user_id` BIGINT NOT NULL COMMENT '关联用户ID（管理员为0）',
  `expire_at` DATETIME NULL DEFAULT NULL COMMENT '过期时间（NULL=永久）',
  `last_warn_at` DATETIME NULL DEFAULT NULL COMMENT '最近一次失效预警发送时间',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态（1=有效，0=失效）',
  `click_count` BIGINT NOT NULL DEFAULT 0 COMMENT '点击量统计',
  `is_hot` TINYINT DEFAULT 0 COMMENT '是否为热点短码（1=是，日访问≥1000）',
  `is_custom` TINYINT DEFAULT 0 COMMENT '是否为自定义短码（1=是）',
  `created_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间',
  `deleted_at` DATETIME NULL DEFAULT NULL COMMENT '软删除标记（非NULL表示已删除）',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_short_code` (`short_code`),
  UNIQUE KEY `uk_original_url_md5` (`original_url_md5`),
  UNIQUE KEY `uk_user_url_md5` (`user_id`, `original_url_md5`),
  INDEX `idx_user_id_status` (`user_id`, `status`),
  INDEX `idx_expire_at` (`expire_at`),
  INDEX `idx_deleted_at` (`deleted_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='短链接表';